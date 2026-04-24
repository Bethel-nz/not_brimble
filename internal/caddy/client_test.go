package caddy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"not_brimble/internal/caddy"
)

func TestAddRoute(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("got method %q, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "routes") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var buf strings.Builder
		io.Copy(&buf, r.Body)
		gotBody = buf.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := caddy.NewClient(srv.URL)
	err := c.AddRoute(context.Background(), caddy.Route{
		DeploymentID: "abc123",
		Host:         "abc123.localhost",
		Upstream:     "deploy-abc123:3000",
	})
	if err != nil {
		t.Fatalf("add route: %v", err)
	}
	if !strings.Contains(gotBody, "abc123.localhost") {
		t.Errorf("route body missing subdomain: %s", gotBody)
	}
	if !strings.Contains(gotBody, "deploy-abc123:3000") {
		t.Errorf("route body missing upstream: %s", gotBody)
	}
}

func TestDeleteRoute(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("got method %q, want DELETE", r.Method)
		}
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := caddy.NewClient(srv.URL)
	if err := c.DeleteRoute(context.Background(), "abc123"); err != nil {
		t.Fatalf("delete route: %v", err)
	}
	if gotPath != "/id/deploy-abc123" {
		t.Errorf("got path %q, want /id/deploy-abc123", gotPath)
	}
}
