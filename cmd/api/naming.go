package main

import (
	"path/filepath"
	"strings"
)

// deriveName produces a DNS/Docker-safe slug for a deployment. It's used as
// the image tag, container name, Caddy @id, and subdomain. Format:
//
//	<base>-<suffix>
//
// where <base> comes from the source (repo name for git, filename for
// upload) and <suffix> is a short lowercased chunk of the ULID so repeated
// deploys of the same source each get a unique URL without needing
// collision detection on the server.
func deriveName(sourceType, sourceURL, id string) string {
	base := sanitizeLabel(rawBase(sourceType, sourceURL))
	if base == "" {
		base = "app"
	}
	const baseMax = 40
	if len(base) > baseMax {
		base = base[:baseMax]
	}
	suffix := strings.ToLower(id)
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	return base + "-" + suffix
}

func rawBase(sourceType, sourceURL string) string {
	switch sourceType {
	case "git", "rollback":
		// Rollbacks reuse the original source_url so they group with
		// their siblings in the UI and get a readable name.
		return repoNameFromGitURL(sourceURL)
	case "upload":
		name := filepath.Base(sourceURL)
		name = strings.TrimSuffix(name, ".gz")
		name = strings.TrimSuffix(name, ".tar")
		name = strings.TrimSuffix(name, ".tgz")
		return name
	}
	return ""
}

func repoNameFromGitURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")
	if i := strings.LastIndex(url, "/"); i != -1 {
		return url[i+1:]
	}
	if i := strings.LastIndex(url, ":"); i != -1 {
		return url[i+1:]
	}
	return url
}

// sanitizeLabel reduces an arbitrary string to the DNS intersection:
// lowercase alphanumerics plus hyphens, no leading/trailing hyphen, no
// consecutive hyphens. It also splits CamelCase boundaries so that
// "MyApp" becomes "my-app" and "HTTPServer" becomes "http-server".
func sanitizeLabel(s string) string {
	kebab := splitCamel(s)

	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(kebab) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// splitCamel inserts a hyphen at CamelCase boundaries:
//
//	MyApp        → My-App
//	HTTPServer   → HTTP-Server
//	parseXMLDoc  → parse-XML-Doc
//
// Runs of uppercase letters stay together so that acronyms read naturally
// after lowercasing.
func splitCamel(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rs[i-1]
			prevIsLowerOrDigit := (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')
			nextIsLower := i+1 < len(rs) && rs[i+1] >= 'a' && rs[i+1] <= 'z'
			prevIsUpper := prev >= 'A' && prev <= 'Z'
			if prevIsLowerOrDigit || (prevIsUpper && nextIsLower) {
				b.WriteRune('-')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
