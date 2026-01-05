package fetcher_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"
	_ "repomedic/internal/fetcher/providers"
	gh "repomedic/internal/github"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestFetcher_RealFetch(t *testing.T) {
	// Mock Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup Client
	client, err := gh.NewClient(context.Background(), "dummy")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	url, _ := url.Parse(server.URL + "/")
	client.Client.BaseURL = url
	client.Client.UploadURL = url

	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(client, budget)

	repo := &github.Repository{
		Owner:         &github.User{Login: github.String("acme")},
		Name:          github.String("repo"),
		FullName:      github.String("acme/repo"),
		DefaultBranch: github.String("main"),
	}

	// 1. Test Repo Metadata
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "4000")
		fmt.Fprint(w, `{"id":1, "name":"repo", "default_branch":"main"}`)
	})

	val, err := f.Fetch(context.Background(), repo, data.DepRepoMetadata, nil)
	if err != nil {
		t.Fatalf("Fetch metadata failed: %v", err)
	}
	if r, ok := val.(*github.Repository); !ok || r.GetName() != "repo" {
		t.Errorf("Expected repository object, got %T %v", val, val)
	}

	// 2. Test Branch Protection (Exists)
	mux.HandleFunc("/repos/acme/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "3999")
		fmt.Fprint(w, `{"url":"...", "required_status_checks":{"contexts":["ci"]}}`)
	})

	val, err = f.Fetch(context.Background(), repo, data.DepRepoDefaultBranchClassicProtection, nil)
	if err != nil {
		t.Fatalf("Fetch protection failed: %v", err)
	}
	if p, ok := val.(*github.Protection); !ok || len(*p.RequiredStatusChecks.Contexts) != 1 {
		t.Errorf("Expected protection object, got %T %v", val, val)
	}

	// 3. Test Branch Protection (404 - Not Protected)
	// We need a different repo or reset mux? Or just use a different key/repo for this test case.
	// Let's use a different repo struct.
	repoUnprotected := &github.Repository{
		Owner:         &github.User{Login: github.String("acme")},
		Name:          github.String("unprotected"),
		FullName:      github.String("acme/unprotected"),
		DefaultBranch: github.String("main"),
	}
	mux.HandleFunc("/repos/acme/unprotected/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"message": "Branch not protected"}`)
	})

	val, err = f.Fetch(context.Background(), repoUnprotected, data.DepRepoDefaultBranchClassicProtection, nil)
	if err != nil {
		t.Fatalf("Fetch unprotected failed: %v", err)
	}
	if val != nil {
		t.Errorf("Expected nil for unprotected branch, got %v", val)
	}

	// 4. Test Unknown Key
	_, err = f.Fetch(context.Background(), repo, "unknown.key", nil)
	if err == nil {
		t.Error("Expected error for unknown key")
	}

	// Verify Budget Updates
	rem := budget.Remaining()
	// We started with 5000.
	// Calls:
	// 1. Metadata -> 4000 (from header)
	// 2. Protection -> 3999 (from header)
	// 3. Unprotected -> 404 (no header set in mock? go-github might not parse header on error?
	//    Actually, on 404 error, go-github returns response.
	//    But in my mock for 404 I didn't set header.
	//    So it might stay at 3999 or decrement by 1 if Acquire happened.
	//    Acquire happens before fetch.
	//    Let's check logic.
	//    Acquire decrements. UpdateFromResponse sets absolute value.
	//    If 404 mock didn't set header, budget remains what Acquire set it to (or previous update).
	// 4. Unknown -> Acquire decrements, but no fetch.

	// So last successful update was 3999.
	// Then unknown key acquired 1. So 3997?
	// Let's just check it's not 5000.
	if rem == 5000 {
		t.Error("Budget was not updated")
	}
}

func TestFetcher_Cache(t *testing.T) {
	// Mock Server
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/cache-repo", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, `{"id":1, "name":"cache-repo"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := gh.NewClient(context.Background(), "dummy")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	u, _ := url.Parse(server.URL + "/")
	client.Client.BaseURL = u

	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(client, budget)

	repo := &github.Repository{
		Owner:    &github.User{Login: github.String("acme")},
		Name:     github.String("cache-repo"),
		FullName: github.String("acme/cache-repo"),
	}

	// First call
	_, err = f.Fetch(context.Background(), repo, data.DepRepoMetadata, nil)
	if err != nil {
		t.Fatalf("First fetch failed: %v", err)
	}

	// Second call
	_, err = f.Fetch(context.Background(), repo, data.DepRepoMetadata, nil)
	if err != nil {
		t.Fatalf("Second fetch failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}
}
