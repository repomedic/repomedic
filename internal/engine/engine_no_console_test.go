package engine

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"repomedic/internal/config"
	gh "repomedic/internal/github"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestEngine_Run_NoConsole(t *testing.T) {
	// Setup a mock GitHub server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo1", "full_name":"acme/repo1", "default_branch":"main", "owner":{"login":"acme"}, "visibility":"public"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	cfg := config.New()
	cfg.Targeting.Repos = []string{"acme/repo1"}
	cfg.Output.NoConsole = true
	cfg.Runtime.Concurrency = 1

	// Capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w
	os.Stderr = w

	eng := NewEngine(ghClient)
	// We don't care about the result, just the output
	_ = eng.Run(context.Background(), cfg)

	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	out := buf.String()

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no console output when NoConsole is true; got:\n%s", out)
	}
}

func TestEngine_Run_Console_Default(t *testing.T) {
	// Setup a mock GitHub server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo1", "full_name":"acme/repo1", "default_branch":"main", "owner":{"login":"acme"}, "visibility":"public"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	cfg := config.New()
	cfg.Targeting.Repos = []string{"acme/repo1"}
	cfg.Output.NoConsole = false // Default
	cfg.Runtime.Concurrency = 1

	// Capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w
	os.Stderr = w

	eng := NewEngine(ghClient)
	_ = eng.Run(context.Background(), cfg)

	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	out := buf.String()

	if strings.TrimSpace(out) == "" {
		t.Error("expected console output when NoConsole is false, got empty output")
	}
}
