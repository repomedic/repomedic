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
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-github/v66/github"
)

type testCycleFetcher struct {
	key    data.DependencyKey
	target data.DependencyKey
}

func (t *testCycleFetcher) Key() data.DependencyKey { return t.key }

func (t *testCycleFetcher) Scope() data.FetchScope { return data.ScopeRepo }

func (t *testCycleFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	return f.Fetch(ctx, repo, t.target, nil)
}

type testValueFetcher struct {
	key   data.DependencyKey
	scope data.FetchScope
	calls *int32
}

func (t *testValueFetcher) Key() data.DependencyKey { return t.key }

func (t *testValueFetcher) Scope() data.FetchScope { return t.scope }

func (t *testValueFetcher) Fetch(_ context.Context, _ *github.Repository, _ map[string]string, _ *fetcher.Fetcher) (any, error) {
	atomic.AddInt32(t.calls, 1)
	return "ok", nil
}

const (
	testOrgScopeKey  data.DependencyKey = "test.scope.org"
	testRepoScopeKey data.DependencyKey = "test.scope.repo"
)

var (
	testOrgScopeCalls  int32
	testRepoScopeCalls int32
	testScopeOnce      sync.Once
)

func ensureTestScopeFetchersRegistered() {
	testScopeOnce.Do(func() {
		fetcher.RegisterDataFetcher(&testValueFetcher{key: testOrgScopeKey, scope: data.ScopeOrg, calls: &testOrgScopeCalls})
		fetcher.RegisterDataFetcher(&testValueFetcher{key: testRepoScopeKey, scope: data.ScopeRepo, calls: &testRepoScopeCalls})
	})
}

func newTestClient(t *testing.T, serverURL string) *gh.Client {
	t.Helper()

	client, err := gh.NewClient(context.Background(), "dummy-token")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	baseURL, err := url.Parse(serverURL + "/")
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	client.Client.BaseURL = baseURL
	client.Client.UploadURL = baseURL
	return client
}

func budgetRemaining(b *fetcher.RequestBudget) int {
	return b.Remaining()
}

func TestDataFetcherRegistry_ResolvesKnownKeys(t *testing.T) {
	tests := []struct {
		name string
		key  data.DependencyKey
	}{
		{name: "metadata", key: data.DepRepoMetadata},
		{name: "default branch protection", key: data.DepRepoDefaultBranchClassicProtection},
		{name: "protected branches deletion status", key: data.DepRepoProtectedBranchesDeletionStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := fetcher.ResolveDataFetcher(tt.key); !ok {
				t.Fatalf("expected data fetcher registered for key %q", tt.key)
			}
		})
	}
}

func TestFetcher_Fetch(t *testing.T) {
	// Mock Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup
	client := newTestClient(t, server.URL)

	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(client, budget)

	repo := &github.Repository{
		Owner: &github.User{Login: github.String("acme")},
		Name:  github.String("repo"),
	}

	// Mock Metadata response
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo"}`)
	})

	// Test Fetch with valid key
	val, err := f.Fetch(context.Background(), repo, data.DepRepoMetadata, nil)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if r, ok := val.(*github.Repository); !ok || r.GetName() != "repo" {
		t.Errorf("Expected repo object, got %v", val)
	}

	// Verify budget was acquired (remaining should be 4999)
	if rem := budgetRemaining(budget); rem != 4999 {
		t.Errorf("Expected 4999 remaining, got %d", rem)
	}
}

func TestFetcher_CacheKey_DeterministicParamsOrder(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	callCount := 0
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, `{"id":1, "name":"repo"}`)
	})

	client := newTestClient(t, server.URL)
	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(client, budget)

	repo := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo"), FullName: github.String("acme/repo")}

	paramsA := map[string]string{"b": "2", "a": "1"}
	paramsB := map[string]string{"a": "1", "b": "2"}

	if _, err := f.Fetch(context.Background(), repo, data.DepRepoMetadata, paramsA); err != nil {
		t.Fatalf("Fetch paramsA failed: %v", err)
	}
	if _, err := f.Fetch(context.Background(), repo, data.DepRepoMetadata, paramsB); err != nil {
		t.Fatalf("Fetch paramsB failed: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected 1 API call due to deterministic cache key, got %d", callCount)
	}
}

func TestFetcher_DefaultBranchProtection_FallsBackToMetadataWhenBranchMissing(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	metaCalls := 0
	protectionCalls := 0

	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		metaCalls++
		fmt.Fprint(w, `{"id":1, "name":"repo", "default_branch":"main"}`)
	})
	mux.HandleFunc("/repos/acme/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		protectionCalls++
		fmt.Fprint(w, `{"url":"...", "required_status_checks":{"contexts":["ci"]}}`)
	})

	client := newTestClient(t, server.URL)
	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(client, budget)

	repo := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo"), FullName: github.String("acme/repo")}
	val, err := f.Fetch(context.Background(), repo, data.DepRepoDefaultBranchClassicProtection, nil)
	if err != nil {
		t.Fatalf("Fetch default branch protection failed: %v", err)
	}
	if val == nil {
		t.Fatalf("expected non-nil protection result")
	}
	if metaCalls != 1 {
		t.Fatalf("expected 1 metadata call, got %d", metaCalls)
	}
	if protectionCalls != 1 {
		t.Fatalf("expected 1 protection call, got %d", protectionCalls)
	}
}

func TestFetcher_DefaultBranchProtection_ErrorsWhenMetadataHasNoDefaultBranch(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	metaCalls := 0
	protectionCalls := 0

	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		metaCalls++
		fmt.Fprint(w, `{"id":1, "name":"repo", "default_branch":""}`)
	})
	mux.HandleFunc("/repos/acme/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		protectionCalls++
		fmt.Fprint(w, `{"url":"..."}`)
	})

	client := newTestClient(t, server.URL)
	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(client, budget)

	repo := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo"), FullName: github.String("acme/repo")}
	_, err := f.Fetch(context.Background(), repo, data.DepRepoDefaultBranchClassicProtection, nil)
	if err == nil {
		t.Fatalf("expected error when default branch cannot be resolved")
	}
	if metaCalls != 1 {
		t.Fatalf("expected 1 metadata call, got %d", metaCalls)
	}
	if protectionCalls != 0 {
		t.Fatalf("expected 0 protection calls, got %d", protectionCalls)
	}
}

func TestFetcher_DefaultBranchProtection_DoesNotDoubleFetchMetadata(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	metaCalls := 0
	protectionCalls := 0

	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		metaCalls++
		fmt.Fprint(w, `{"id":1, "name":"repo", "default_branch":"main"}`)
	})
	mux.HandleFunc("/repos/acme/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		protectionCalls++
		fmt.Fprint(w, `{"url":"...", "required_status_checks":{"contexts":["ci"]}}`)
	})

	client := newTestClient(t, server.URL)
	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(client, budget)

	repo := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo"), FullName: github.String("acme/repo")}

	// Prime metadata cache.
	if _, err := f.Fetch(context.Background(), repo, data.DepRepoMetadata, nil); err != nil {
		t.Fatalf("Fetch metadata failed: %v", err)
	}
	if _, err := f.Fetch(context.Background(), repo, data.DepRepoDefaultBranchClassicProtection, nil); err != nil {
		t.Fatalf("Fetch default branch protection failed: %v", err)
	}

	if metaCalls != 1 {
		t.Fatalf("expected 1 metadata call, got %d", metaCalls)
	}
	if protectionCalls != 1 {
		t.Fatalf("expected 1 protection call, got %d", protectionCalls)
	}
}

func TestFetcher_DependencyCycleDetection_SelfCycle(t *testing.T) {
	const selfKey data.DependencyKey = "test.cycle.self"
	fetcher.RegisterDataFetcher(&testCycleFetcher{key: selfKey, target: selfKey})

	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()
	client := newTestClient(t, server.URL)
	f := fetcher.NewFetcher(client, fetcher.NewRequestBudget())

	repo := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo"), FullName: github.String("acme/repo")}
	_, err := f.Fetch(context.Background(), repo, selfKey, nil)
	if err == nil {
		t.Fatalf("expected cycle detection error")
	}
}

func TestFetcher_DependencyCycleDetection_MutualCycle(t *testing.T) {
	const aKey data.DependencyKey = "test.cycle.a"
	const bKey data.DependencyKey = "test.cycle.b"
	fetcher.RegisterDataFetcher(&testCycleFetcher{key: aKey, target: bKey})
	fetcher.RegisterDataFetcher(&testCycleFetcher{key: bKey, target: aKey})

	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()
	client := newTestClient(t, server.URL)
	f := fetcher.NewFetcher(client, fetcher.NewRequestBudget())

	repo := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo"), FullName: github.String("acme/repo")}
	_, err := f.Fetch(context.Background(), repo, aKey, nil)
	if err == nil {
		t.Fatalf("expected cycle detection error")
	}
}

func TestFetcher_FetchScope_Org_DedupesAcrossReposSameOrg(t *testing.T) {
	ensureTestScopeFetchersRegistered()
	atomic.StoreInt32(&testOrgScopeCalls, 0)

	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()
	client := newTestClient(t, server.URL)
	f := fetcher.NewFetcher(client, fetcher.NewRequestBudget())

	repoA := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo-a"), FullName: github.String("acme/repo-a")}
	repoB := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo-b"), FullName: github.String("acme/repo-b")}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, r := range []*github.Repository{repoA, repoB} {
		wg.Add(1)
		go func(repo *github.Repository) {
			defer wg.Done()
			<-start
			val, err := f.Fetch(context.Background(), repo, testOrgScopeKey, nil)
			if err != nil {
				t.Errorf("Fetch failed: %v", err)
				return
			}
			if val != "ok" {
				t.Errorf("got %v, want %v", val, "ok")
			}
		}(r)
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&testOrgScopeCalls); got != 1 {
		t.Fatalf("expected 1 fetch call for org-scoped dep across same org, got %d", got)
	}
}

func TestFetcher_FetchScope_Org_DoesNotDedupeAcrossDifferentOrgs(t *testing.T) {
	ensureTestScopeFetchersRegistered()
	atomic.StoreInt32(&testOrgScopeCalls, 0)

	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()
	client := newTestClient(t, server.URL)
	f := fetcher.NewFetcher(client, fetcher.NewRequestBudget())

	repoA := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo-a"), FullName: github.String("acme/repo-a")}
	repoB := &github.Repository{Owner: &github.User{Login: github.String("other")}, Name: github.String("repo-b"), FullName: github.String("other/repo-b")}

	if _, err := f.Fetch(context.Background(), repoA, testOrgScopeKey, nil); err != nil {
		t.Fatalf("Fetch repoA failed: %v", err)
	}
	if _, err := f.Fetch(context.Background(), repoB, testOrgScopeKey, nil); err != nil {
		t.Fatalf("Fetch repoB failed: %v", err)
	}

	if got := atomic.LoadInt32(&testOrgScopeCalls); got != 2 {
		t.Fatalf("expected 2 fetch calls for org-scoped dep across different orgs, got %d", got)
	}
}

func TestFetcher_FetchScope_Repo_DoesNotDedupeAcrossRepos(t *testing.T) {
	ensureTestScopeFetchersRegistered()
	atomic.StoreInt32(&testRepoScopeCalls, 0)

	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()
	client := newTestClient(t, server.URL)
	f := fetcher.NewFetcher(client, fetcher.NewRequestBudget())

	repoA := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo-a"), FullName: github.String("acme/repo-a")}
	repoB := &github.Repository{Owner: &github.User{Login: github.String("acme")}, Name: github.String("repo-b"), FullName: github.String("acme/repo-b")}

	if _, err := f.Fetch(context.Background(), repoA, testRepoScopeKey, nil); err != nil {
		t.Fatalf("Fetch repoA failed: %v", err)
	}
	if _, err := f.Fetch(context.Background(), repoB, testRepoScopeKey, nil); err != nil {
		t.Fatalf("Fetch repoB failed: %v", err)
	}

	if got := atomic.LoadInt32(&testRepoScopeCalls); got != 2 {
		t.Fatalf("expected 2 fetch calls for repo-scoped dep across repos, got %d", got)
	}
}
