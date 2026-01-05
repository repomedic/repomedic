package github

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveAuthToken(t *testing.T) {
	t.Run("explicit token wins", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "env-token")
		t.Setenv("PATH", t.TempDir())

		tok, src, err := ResolveAuthToken(context.Background(), " explicit ")
		if err != nil {
			t.Fatalf("ResolveAuthToken error: %v", err)
		}
		if tok != "explicit" {
			t.Fatalf("want explicit, got %q", tok)
		}
		if src != AuthTokenSourceExplicit {
			t.Fatalf("want %q, got %q", AuthTokenSourceExplicit, src)
		}
	})

	t.Run("env token used", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "env-token")
		t.Setenv("PATH", t.TempDir())

		tok, src, err := ResolveAuthToken(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveAuthToken error: %v", err)
		}
		if tok != "env-token" {
			t.Fatalf("want env-token, got %q", tok)
		}
		if src != AuthTokenSourceEnv {
			t.Fatalf("want %q, got %q", AuthTokenSourceEnv, src)
		}
	})

	t.Run("gh token used when env empty", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("test uses a shell script gh stub")
		}

		tmp := t.TempDir()
		ghPath := filepath.Join(tmp, "gh")
		if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho gh-token\n"), 0o755); err != nil {
			t.Fatalf("WriteFile gh stub failed: %v", err)
		}

		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("PATH", tmp)

		tok, src, err := ResolveAuthToken(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveAuthToken error: %v", err)
		}
		if tok != "gh-token" {
			t.Fatalf("want gh-token, got %q", tok)
		}
		if src != AuthTokenSourceGitHubCL {
			t.Fatalf("want %q, got %q", AuthTokenSourceGitHubCL, src)
		}
	})

	t.Run("empty when neither env nor gh", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("PATH", t.TempDir())

		tok, src, err := ResolveAuthToken(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveAuthToken error: %v", err)
		}
		if tok != "" {
			t.Fatalf("want empty token, got %q", tok)
		}
		if src != "" {
			t.Fatalf("want empty source, got %q", src)
		}
	})

	t.Run("gh invalid token output returns error", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("test uses a shell script gh stub")
		}

		tmp := t.TempDir()
		ghPath := filepath.Join(tmp, "gh")
		if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nprintf 'line1\\nline2\\n'\n"), 0o755); err != nil {
			t.Fatalf("WriteFile gh stub failed: %v", err)
		}

		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("PATH", tmp)

		_, _, err := ResolveAuthToken(context.Background(), "")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("context canceled propagates error when using gh", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("test uses a shell script gh stub")
		}

		tmp := t.TempDir()
		ghPath := filepath.Join(tmp, "gh")
		if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho gh-token\n"), 0o755); err != nil {
			t.Fatalf("WriteFile gh stub failed: %v", err)
		}

		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("PATH", tmp)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := ResolveAuthToken(ctx, "")
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}
