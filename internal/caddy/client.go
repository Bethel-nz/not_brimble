package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	adminURL   string
	httpClient *http.Client
}

func NewClient(adminURL string) *Client {
	// Force tcp4 for the dialer. Go's default resolver on Linux issues
	// parallel A+AAAA lookups; Docker's embedded DNS (127.0.0.11) can
	// return a malformed AAAA response that Go surfaces as "no such host"
	// even though the A record resolves fine. Constraining the network to
	// tcp4 skips the AAAA query entirely.
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", addr)
		},
	}
	return &Client{
		adminURL:   adminURL,
		httpClient: &http.Client{Transport: transport, Timeout: 15 * time.Second},
	}
}

type Route struct {
	DeploymentID string // lowercase; used to build the stable @id tag
	Host         string // e.g. "<id>.localhost"
	Upstream     string // e.g. "deploy-<id>:3000"
}

type caddyRoute struct {
	ID     string        `json:"@id"`
	Match  []caddyMatch  `json:"match"`
	Handle []caddyHandle `json:"handle"`
}

type caddyMatch struct {
	Host []string `json:"host"`
}

type caddyHandle struct {
	Handler   string          `json:"handler"`
	Upstreams []caddyUpstream `json:"upstreams"`
}

type caddyUpstream struct {
	Dial string `json:"dial"`
}

// AddRoute registers a reverse-proxy route in Caddy with a stable @id tag
// so later deletes can target it directly without caring about position.
func (c *Client) AddRoute(ctx context.Context, route Route) error {
	routeID := "deploy-" + route.DeploymentID
	r := caddyRoute{
		ID:    routeID,
		Match: []caddyMatch{{Host: []string{route.Host}}},
		Handle: []caddyHandle{{
			Handler:   "reverse_proxy",
			Upstreams: []caddyUpstream{{Dial: route.Upstream}},
		}},
	}

	// The /... suffix on the admin-API path means "append": Caddy expects
	// the body to be an array whose elements are merged into the target.
	// Wrap the single route so the server can unpack it.
	body, err := json.Marshal([]caddyRoute{r})
	if err != nil {
		return fmt.Errorf("marshal route: %w", err)
	}

	url := c.adminURL + "/config/apps/http/servers/srv0/routes/..."
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy add route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy add route: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// DeleteRoute removes a previously-added route by its stable @id tag.
// A missing route (404) is treated as success so deletes are idempotent.
func (c *Client) DeleteRoute(ctx context.Context, deploymentID string) error {
	routeID := "deploy-" + deploymentID
	url := c.adminURL + "/id/" + routeID

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy delete route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("caddy delete route: status %d: %s", resp.StatusCode, string(b))
}
