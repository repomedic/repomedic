package providers_test

import (
	"context"
	"strings"
	"testing"

	"repomedic/internal/data"
	"repomedic/internal/fetcher"
	_ "repomedic/internal/fetcher/providers"
	gh "repomedic/internal/github"

	"github.com/google/go-github/v81/github"
)

func newTestFetcher(t *testing.T) *fetcher.Fetcher {
	t.Helper()

	client, err := gh.NewClient(context.Background(), "dummy-token")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	budget := fetcher.NewRequestBudget()
	return fetcher.NewFetcher(client, budget)
}

func TestReposScannedFetcher_Registration(t *testing.T) {
	df, ok := fetcher.ResolveDataFetcher(data.DepReposScanned)
	if !ok {
		t.Fatal("expected data fetcher registered for DepReposScanned")
	}
	if df.Key() != data.DepReposScanned {
		t.Errorf("Key() = %q, want %q", df.Key(), data.DepReposScanned)
	}
	if df.Scope() != data.ScopeOrg {
		t.Errorf("Scope() = %q, want %q", df.Scope(), data.ScopeOrg)
	}
}

func TestReposScannedFetcher_WithInjectedList(t *testing.T) {
	f := newTestFetcher(t)

	// Inject scanned repos
	expectedRepos := []*github.Repository{
		{FullName: github.Ptr("org/repo1")},
		{FullName: github.Ptr("org/repo2")},
	}
	f.SetScannedRepos(expectedRepos)

	// Fetch using a representative repo (the key is org-scoped, so repo is just context)
	repo := &github.Repository{
		Owner: &github.User{Login: github.Ptr("org")},
		Name:  github.Ptr("repo1"),
	}

	result, err := f.Fetch(context.Background(), repo, data.DepReposScanned, nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	repos, ok := result.([]*github.Repository)
	if !ok {
		t.Fatalf("Fetch returned type %T, want []*github.Repository", result)
	}

	if len(repos) != len(expectedRepos) {
		t.Fatalf("got %d repos, want %d", len(repos), len(expectedRepos))
	}

	for i, r := range repos {
		if r.GetFullName() != expectedRepos[i].GetFullName() {
			t.Errorf("repos[%d].FullName = %q, want %q", i, r.GetFullName(), expectedRepos[i].GetFullName())
		}
	}
}

func TestReposScannedFetcher_WithoutInjectedList_ReturnsError(t *testing.T) {
	f := newTestFetcher(t)
	// Do NOT call SetScannedRepos

	repo := &github.Repository{
		Owner: &github.User{Login: github.Ptr("org")},
		Name:  github.Ptr("repo1"),
	}

	_, err := f.Fetch(context.Background(), repo, data.DepReposScanned, nil)
	if err == nil {
		t.Fatal("Fetch should return error when scanned repos not injected")
	}

	expectedMsg := "scanned repos not available"
	if err.Error() != expectedMsg && !strings.Contains(err.Error(), "scanned repos not available") {
		t.Errorf("error = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

func TestReposScannedFetcher_WithEmptyList(t *testing.T) {
	f := newTestFetcher(t)

	// Inject empty list (valid case: no repos discovered)
	f.SetScannedRepos([]*github.Repository{})

	repo := &github.Repository{
		Owner: &github.User{Login: github.Ptr("org")},
		Name:  github.Ptr("repo1"),
	}

	result, err := f.Fetch(context.Background(), repo, data.DepReposScanned, nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	repos, ok := result.([]*github.Repository)
	if !ok {
		t.Fatalf("Fetch returned type %T, want []*github.Repository", result)
	}

	if len(repos) != 0 {
		t.Errorf("got %d repos, want 0", len(repos))
	}
}
