package main

import "testing"

func TestDeriveName(t *testing.T) {
	id := "01KPVFB2H5XPTF56MK6148MY9Y"
	tests := []struct {
		name       string
		sourceType string
		sourceURL  string
		want       string
	}{
		{"git https with .git", "git", "https://github.com/user/my-app.git", "my-app-48my9y"},
		{"git https no .git", "git", "https://github.com/user/my-app", "my-app-48my9y"},
		{"git ssh", "git", "git@github.com:user/MyApp.git", "my-app-48my9y"},
		{"camelcase", "git", "https://github.com/user/MyApp.git", "my-app-48my9y"},
		{"acronym", "git", "https://github.com/user/HTTPServer", "http-server-48my9y"},
		{"mixed acronym", "git", "https://github.com/user/parseXMLDoc", "parse-xml-doc-48my9y"},
		{"dots in repo", "git", "https://github.com/user/my.awesome.app", "my-awesome-app-48my9y"},
		{"upload tar.gz", "upload", "/tmp/uploads/my-project.tar.gz", "my-project-48my9y"},
		{"upload .tgz", "upload", "/tmp/uploads/MyProject.tgz", "my-project-48my9y"},
		{"empty git url", "git", "", "app-48my9y"},
		{"garbage chars", "git", "https://host/!!!", "app-48my9y"},
		{"very long base", "git", "https://host/" + longString(80), "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-48my9y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveName(tt.sourceType, tt.sourceURL, id)
			if got != tt.want {
				t.Errorf("deriveName(%q, %q) = %q; want %q", tt.sourceType, tt.sourceURL, got, tt.want)
			}
		})
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
