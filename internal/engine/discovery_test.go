package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"repomedic/internal/config"
	gh "repomedic/internal/github"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) failed: %v", raw, err)
	}
	return u
}

func newTestGitHubClient(t *testing.T, serverURL string) *gh.Client {
	t.Helper()
	client, err := gh.NewClient(context.Background(), "dummy")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	base := mustParseURL(t, serverURL+"/")
	client.Client.BaseURL = base
	client.Client.UploadURL = base
	return client
}

func TestResolveRepos(t *testing.T) {
	t.Run("explicit repo selector", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		mux.HandleFunc("/repos/acme/foo", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id":1, "name":"foo", "owner":{"login":"acme"}}`)
		})

		cfg := config.New()
		cfg.Targeting.Repos = []string{"acme/foo"}
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 || refs[0].Owner != "acme" || refs[0].Name != "foo" {
			t.Fatalf("Expected 1 repo 'acme/foo', got %v", refs)
		}
	})

	t.Run("explicit repo selector (GitHub URL form)", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		mux.HandleFunc("/repos/acme/foo", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id":1, "name":"foo", "owner":{"login":"acme"}}`)
		})

		cfg := config.New()
		cfg.Targeting.Repos = []string{"https://github.com/acme/foo"}
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 || refs[0].Owner != "acme" || refs[0].Name != "foo" {
			t.Fatalf("Expected 1 repo 'acme/foo', got %v", refs)
		}
	})

	t.Run("explicit repos are deduped", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		mux.HandleFunc("/repos/acme/foo", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id":1, "name":"foo", "owner":{"login":"acme"}}`)
		})

		cfg := config.New()
		cfg.Targeting.Repos = []string{"acme/foo", "acme/foo"}
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 {
			t.Fatalf("Expected 1 repo after dedupe, got %d (%v)", len(refs), refs)
		}
	})

	t.Run("org discovery", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"id":2, "name":"bar", "owner":{"login":"test-org"}}]`)
		})

		cfg := config.New()
		cfg.Targeting.Org = "test-org"
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 || refs[0].Owner != "test-org" || refs[0].Name != "bar" {
			t.Fatalf("Expected 1 repo 'test-org/bar', got %v", refs)
		}
	})

	t.Run("org discovery (org provided as GitHub URL)", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"id":2, "name":"bar", "owner":{"login":"test-org"}}]`)
		})

		cfg := config.New()
		cfg.Targeting.Org = "https://github.com/test-org"
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 || refs[0].Owner != "test-org" || refs[0].Name != "bar" {
			t.Fatalf("Expected 1 repo 'test-org/bar', got %v", refs)
		}
	})

	t.Run("user discovery uses authenticated endpoint when user matches token owner", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		userMeCalls := 0
		userReposCalls := 0
		authedUserReposCalls := 0

		mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
			userMeCalls++
			fmt.Fprint(w, `{"login":"test-user"}`)
		})
		mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
			authedUserReposCalls++
			fmt.Fprint(w, `[{"id":3, "name":"baz", "full_name":"test-user/baz", "owner":{"login":"test-user"}}]`)
		})
		mux.HandleFunc("/users/test-user/repos", func(w http.ResponseWriter, r *http.Request) {
			userReposCalls++
			fmt.Fprint(w, `[{"id":3, "name":"baz", "full_name":"test-user/baz", "owner":{"login":"test-user"}}]`)
		})

		cfg := config.New()
		cfg.Targeting.User = "test-user"
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 || refs[0].Owner != "test-user" || refs[0].Name != "baz" {
			t.Fatalf("Expected 1 repo 'test-user/baz', got %v", refs)
		}
		if userMeCalls != 1 {
			t.Fatalf("expected /user to be called once, got %d", userMeCalls)
		}
		if authedUserReposCalls != 1 {
			t.Fatalf("expected /user/repos to be called once, got %d", authedUserReposCalls)
		}
		if userReposCalls != 0 {
			t.Fatalf("expected /users/test-user/repos not to be called when authed user matches, got %d", userReposCalls)
		}
	})

	t.Run("org discovery + --repos include filter", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[
				{"id":10, "name":"payments-service", "full_name":"test-org/payments-service", "owner":{"login":"test-org"}},
				{"id":11, "name":"billing-api", "full_name":"test-org/billing-api", "owner":{"login":"test-org"}}
			]`)
		})

		cfg := config.New()
		cfg.Targeting.Org = "test-org"
		cfg.Targeting.Repos = []string{"*-service"}
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 || refs[0].Owner != "test-org" || refs[0].Name != "payments-service" {
			t.Fatalf("Expected 1 repo 'test-org/payments-service', got %v", refs)
		}
	})

	t.Run("org discovery + --repos include filter (GitHub URL form)", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		client := newTestGitHubClient(t, server.URL)

		mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[
				{"id":10, "name":"payments-service", "full_name":"test-org/payments-service", "owner":{"login":"test-org"}},
				{"id":11, "name":"billing-api", "full_name":"test-org/billing-api", "owner":{"login":"test-org"}}
			]`)
		})

		cfg := config.New()
		cfg.Targeting.Org = "test-org"
		cfg.Targeting.Repos = []string{"https://github.com/test-org/billing-api"}
		refs, err := ResolveRepos(context.Background(), client, cfg)
		if err != nil {
			t.Fatalf("ResolveRepos failed: %v", err)
		}
		if len(refs) != 1 || refs[0].Owner != "test-org" || refs[0].Name != "billing-api" {
			t.Fatalf("Expected 1 repo 'test-org/billing-api', got %v", refs)
		}
	})
}

func TestResolveRepos_GlobRequiresScope(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	callCount := 0
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusTeapot)
	})

	client := newTestGitHubClient(t, server.URL)

	cfg := config.New()
	cfg.Targeting.Repos = []string{"acme/*"}

	_, err := ResolveRepos(context.Background(), client, cfg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "contains glob characters") {
		t.Fatalf("expected glob scope error, got: %v", err)
	}
	if callCount != 0 {
		t.Fatalf("expected no network calls for glob selector without scope, got %d", callCount)
	}
}

func TestResolveRepos_GlobFiltersInOrgScope(t *testing.T) {
	// Provide two repos in org listing and filter down to one via a full-name selector.
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestGitHubClient(t, server.URL)

	mux.HandleFunc("/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": 1, "name": "a-service", "full_name": "acme/a-service", "owner": {"login": "acme"}},
			{"id": 2, "name": "b", "full_name": "acme/b", "owner": {"login": "acme"}}
		]`))
	})

	cfg := config.New()
	cfg.Targeting.Org = "acme"
	cfg.Targeting.Repos = []string{"acme/*-service"}

	refs, err := ResolveRepos(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("ResolveRepos returned error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(refs))
	}
	if refs[0].Owner != "acme" || refs[0].Name != "a-service" {
		t.Fatalf("unexpected repo: %+v", refs[0])
	}
}

func TestResolveRepos_OrgDiscovery_IsBoundedByMaxRepos(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	requestCount := 0
	maxPages := 3
	mux.HandleFunc("/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				page = n
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if page < maxPages {
			next := page + 1
			w.Header().Set("Link", fmt.Sprintf("<%s/orgs/acme/repos?page=%d>; rel=\"next\", <%s/orgs/acme/repos?page=%d>; rel=\"last\"", server.URL, next, server.URL, maxPages))
		}

		// Return 100 repos per page.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("["))
		for i := 0; i < 100; i++ {
			id := int64((page-1)*100 + i + 1)
			name := fmt.Sprintf("repo-%04d", id)
			if i > 0 {
				_, _ = w.Write([]byte(","))
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%d,"name":"%s","full_name":"acme/%s","owner":{"login":"acme"}}`, id, name, name)))
		}
		_, _ = w.Write([]byte("]"))
	})

	client := newTestGitHubClient(t, server.URL)

	cfg := config.New()
	cfg.Targeting.Org = "acme"
	cfg.Targeting.MaxRepos = 250

	refs, err := ResolveRepos(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("ResolveRepos returned error: %v", err)
	}
	if len(refs) != 250 {
		t.Fatalf("expected %d repos, got %d", 250, len(refs))
	}
	if requestCount != 3 {
		t.Fatalf("expected 3 requests (pages 1-3), got %d", requestCount)
	}
}

func TestResolveRepos_OrgDiscovery_IsBoundedByDefaultLimit(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	requestCount := 0
	maxPages := defaultOrgDiscoveryRepoLimit / 100
	mux.HandleFunc("/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				page = n
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if page < maxPages {
			next := page + 1
			w.Header().Set("Link", fmt.Sprintf("<%s/orgs/acme/repos?page=%d>; rel=\"next\", <%s/orgs/acme/repos?page=%d>; rel=\"last\"", server.URL, next, server.URL, maxPages))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("["))
		for i := 0; i < 100; i++ {
			id := int64((page-1)*100 + i + 1)
			name := fmt.Sprintf("repo-%04d", id)
			if i > 0 {
				_, _ = w.Write([]byte(","))
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%d,"name":"%s","full_name":"acme/%s","owner":{"login":"acme"}}`, id, name, name)))
		}
		_, _ = w.Write([]byte("]"))
	})

	client := newTestGitHubClient(t, server.URL)

	cfg := config.New()
	cfg.Targeting.Org = "acme"
	cfg.Targeting.MaxRepos = 0

	refs, err := ResolveRepos(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("ResolveRepos returned error: %v", err)
	}
	if len(refs) != defaultOrgDiscoveryRepoLimit {
		t.Fatalf("expected default limit %d repos, got %d", defaultOrgDiscoveryRepoLimit, len(refs))
	}
	// With per_page=100, default limit 1000 should require 10 pages.
	if requestCount != maxPages {
		t.Fatalf("expected %d requests, got %d", maxPages, requestCount)
	}
}

func TestNormalizeRepoSelector(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "trims whitespace",
			in:   "  acme/foo  ",
			want: "acme/foo",
		},
		{
			name: "github.com without scheme",
			in:   "github.com/acme/foo",
			want: "acme/foo",
		},
		{
			name: "www.github.com without scheme",
			in:   "www.github.com/acme/foo",
			want: "acme/foo",
		},
		{
			name: "https URL",
			in:   "https://github.com/acme/foo",
			want: "acme/foo",
		},
		{
			name: "https URL with .git suffix",
			in:   "https://github.com/acme/foo.git",
			want: "acme/foo",
		},
		{
			name: "https URL with extra path segments",
			in:   "https://github.com/acme/foo/tree/main",
			want: "acme/foo",
		},
		{
			name: "ssh URL",
			in:   "git@github.com:acme/foo.git",
			want: "acme/foo",
		},
		{
			name:      "reject non-github host",
			in:        "https://gitlab.com/acme/foo",
			wantErr:   true,
			errSubstr: "expected owner/name",
		},
		{
			name:      "reject github URL missing repo",
			in:        "https://github.com/acme",
			wantErr:   true,
			errSubstr: "expected owner/name",
		},
		{
			name:      "reject ssh missing repo",
			in:        "git@github.com:acme",
			wantErr:   true,
			errSubstr: "expected owner/name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRepoSelector(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tt.errSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeRepoSelector returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNormalizeAccountSelector(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "trims whitespace",
			in:   "  acme  ",
			want: "acme",
		},
		{
			name: "plain login",
			in:   "octocat",
			want: "octocat",
		},
		{
			name: "github.com without scheme",
			in:   "github.com/acme",
			want: "acme",
		},
		{
			name: "www.github.com without scheme",
			in:   "www.github.com/acme",
			want: "acme",
		},
		{
			name: "https org URL",
			in:   "https://github.com/acme",
			want: "acme",
		},
		{
			name: "https orgs URL",
			in:   "https://github.com/orgs/acme",
			want: "acme",
		},
		{
			name: "https users URL",
			in:   "https://github.com/users/octocat",
			want: "octocat",
		},
		{
			name: "https users URL with extra segments",
			in:   "https://github.com/users/octocat/repositories",
			want: "octocat",
		},
		{
			name:      "reject raw with slash",
			in:        "acme/foo",
			wantErr:   true,
			errSubstr: "\"acme/foo\"",
		},
		{
			name:      "reject non-github host",
			in:        "https://gitlab.com/acme",
			wantErr:   true,
			errSubstr: "\"https://gitlab.com/acme\"",
		},
		{
			name:      "reject github root URL",
			in:        "https://github.com/",
			wantErr:   true,
			errSubstr: "\"https://github.com/\"",
		},
		{
			name:      "reject orgs missing org",
			in:        "https://github.com/orgs/",
			wantErr:   true,
			errSubstr: "\"https://github.com/orgs/\"",
		},
		{
			name:      "reject users missing user",
			in:        "https://github.com/users/",
			wantErr:   true,
			errSubstr: "\"https://github.com/users/\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAccountSelector(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tt.errSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeAccountSelector returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestDedupeRefs(t *testing.T) {
	newRepo := func(fullName string) *github.Repository {
		owner := ""
		name := ""
		if parts := strings.Split(fullName, "/"); len(parts) == 2 {
			owner = parts[0]
			name = parts[1]
		}
		return &github.Repository{
			FullName: github.String(fullName),
			Name:     github.String(name),
			Owner:    &github.User{Login: github.String(owner)},
		}
	}

	t.Run("dedupes by ID when present", func(t *testing.T) {
		in := []RepositoryRef{{ID: 1}, {ID: 1}, {ID: 2}, {ID: 2}, {ID: 3}}
		out := dedupeRefs(in)
		if len(out) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(out))
		}
		if out[0].ID != 1 || out[1].ID != 2 || out[2].ID != 3 {
			t.Fatalf("unexpected output order/IDs: %+v", out)
		}
	})

	t.Run("dedupes by repo full_name when ID is not present", func(t *testing.T) {
		in := []RepositoryRef{
			{Repo: newRepo("acme/foo")},
			{Repo: newRepo("acme/foo")},
			{Repo: newRepo("acme/bar")},
			{Repo: newRepo("acme/bar")},
		}
		out := dedupeRefs(in)
		if len(out) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(out))
		}
		if out[0].Repo == nil || out[0].Repo.GetFullName() != "acme/foo" {
			t.Fatalf("expected first repo to be acme/foo, got: %+v", out[0])
		}
		if out[1].Repo == nil || out[1].Repo.GetFullName() != "acme/bar" {
			t.Fatalf("expected second repo to be acme/bar, got: %+v", out[1])
		}
	})

	t.Run("dedupes by Owner/Name when neither ID nor Repo is present", func(t *testing.T) {
		in := []RepositoryRef{
			{Owner: "acme", Name: "foo"},
			{Owner: "acme", Name: "foo"},
			{Owner: "acme", Name: "bar"},
			{Owner: "acme", Name: "bar"},
		}
		out := dedupeRefs(in)
		if len(out) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(out))
		}
		if out[0].Owner != "acme" || out[0].Name != "foo" {
			t.Fatalf("expected first entry to be acme/foo, got: %+v", out[0])
		}
		if out[1].Owner != "acme" || out[1].Name != "bar" {
			t.Fatalf("expected second entry to be acme/bar, got: %+v", out[1])
		}
	})

	t.Run("keeps entries when no stable key can be constructed", func(t *testing.T) {
		in := []RepositoryRef{{}, {}}
		out := dedupeRefs(in)
		if len(out) != 2 {
			t.Fatalf("expected 2 entries to be kept, got %d", len(out))
		}
	})
}
