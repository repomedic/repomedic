package github

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	// Test with explicit token
	ctx := context.Background()
	client, err := NewClient(ctx, "test-token")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client.Client == nil {
		t.Error("Expected client to be initialized with explicit token")
	}

	// Test with env token via resolver (NewClient does not read env vars).
	t.Setenv("GITHUB_TOKEN", "env-token")
	tok, _, err := ResolveAuthToken(ctx, "")
	if err != nil {
		t.Fatalf("ResolveAuthToken failed: %v", err)
	}
	client, err = NewClient(ctx, tok)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client.Client == nil {
		t.Error("Expected client to be initialized with resolved env token")
	}

	// Test with no token (should still init client, just unauthenticated)
	t.Setenv("GITHUB_TOKEN", "")
	client, err = NewClient(ctx, "")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client.Client == nil {
		t.Error("Expected client to be initialized even without token")
	}
}

func TestNewClient_NilContextReturnsError(t *testing.T) {
	var nilCtx context.Context
	_, err := NewClient(nilCtx, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "ctx is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClient_WithVerbose_LogsAndAuthHeader(t *testing.T) {
	ctx := context.Background()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(server.Close)

	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse url: %v", err)
		}
		return u
	}

	// Unauthenticated client should still log when verbose.
	{
		var buf bytes.Buffer
		c, err := NewClient(ctx, "", WithVerbose(true, &buf))
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		c.Client.BaseURL = parse(server.URL + "/")
		c.Client.UploadURL = parse(server.URL + "/")

		req, err := c.Client.NewRequest("GET", "/rate_limit", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		_, err = c.Client.Do(ctx, req, nil)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if !strings.Contains(buf.String(), "[verbose] github api: GET") {
			t.Fatalf("expected verbose log, got: %q", buf.String())
		}
		if gotAuth != "" {
			t.Fatalf("expected no Authorization header, got %q", gotAuth)
		}
	}

	// Authenticated client should send Authorization header.
	{
		gotAuth = ""
		var buf bytes.Buffer
		c, err := NewClient(ctx, "test-token", WithVerbose(true, &buf))
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		c.Client.BaseURL = parse(server.URL + "/")
		c.Client.UploadURL = parse(server.URL + "/")

		req, err := c.Client.NewRequest("GET", "/rate_limit", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		_, err = c.Client.Do(ctx, req, nil)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if !strings.Contains(buf.String(), "[verbose] github api: GET") {
			t.Fatalf("expected verbose log, got: %q", buf.String())
		}
		if gotAuth == "" {
			t.Fatalf("expected Authorization header to be set")
		}
		if !strings.Contains(gotAuth, "test-token") {
			t.Fatalf("expected Authorization header to contain token, got %q", gotAuth)
		}
	}
}
