package fetcher

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	gh "repomedic/internal/github"
	"sort"
	"strings"

	"github.com/google/go-github/v66/github"
)

type Fetcher struct {
	client       *gh.Client
	budget       *RequestBudget
	group        Group
	cache        *Cache
	scannedRepos []*github.Repository
}

type fetchChainKey struct{}

func NewFetcher(client *gh.Client, budget *RequestBudget) *Fetcher {
	return &Fetcher{
		client: client,
		budget: budget,
		cache:  NewCache(),
	}
}

func (f *Fetcher) Budget() *RequestBudget {
	return f.budget
}

func (f *Fetcher) Client() *gh.Client {
	return f.client
}

// SetScannedRepos injects the list of repositories discovered for the current scan.
// This must be called by the engine after discovery but before rule evaluation begins.
// It enables org-scoped fetchers (like DepReposScanned) to access the discovered list
// without making additional GitHub API calls.
func (f *Fetcher) SetScannedRepos(repos []*github.Repository) {
	f.scannedRepos = repos
}

// ScannedRepos returns the list of repositories discovered for the current scan.
// Returns nil if SetScannedRepos has not been called.
func (f *Fetcher) ScannedRepos() []*github.Repository {
	return f.scannedRepos
}

func (f *Fetcher) Fetch(ctx context.Context, repo *github.Repository, key data.DependencyKey, params map[string]string) (any, error) {
	if ctx == nil {
		return nil, fmt.Errorf("Fetch: nil context")
	}
	if f == nil {
		return nil, fmt.Errorf("Fetch: nil Fetcher")
	}
	if f.client == nil || f.client.Client == nil {
		return nil, fmt.Errorf("Fetch: nil GitHub client (use NewFetcher)")
	}
	if f.budget == nil {
		return nil, fmt.Errorf("Fetch: nil request budget (use NewFetcher)")
	}
	if f.cache == nil {
		return nil, fmt.Errorf("Fetch: nil cache (use NewFetcher)")
	}
	if repo == nil {
		return nil, fmt.Errorf("Fetch: nil repo")
	}
	if key == "" {
		return nil, fmt.Errorf("Fetch: empty dependency key")
	}
	if repo.GetOwner().GetLogin() == "" || repo.GetName() == "" {
		return nil, fmt.Errorf("Fetch: repo owner/name is required")
	}

	fetchImpl, ok := ResolveDataFetcher(key)
	if !ok {
		return nil, fmt.Errorf("unsupported dependency key: %s", key)
	}

	// Cache key (must be deterministic)
	flightKey, err := makeFlightKey(repo, fetchImpl.Scope(), key, params)
	if err != nil {
		return nil, err
	}

	ctx, err = withFetchChain(ctx, flightKey)
	if err != nil {
		return nil, err
	}

	// Cache lookup
	if val, ok := f.cache.Get(flightKey); ok {
		return val, nil
	}

	// Single-flight (dedupe concurrent identical requests)
	val, err, _ := f.group.Do(flightKey, func() (interface{}, error) {
		// Fetch
		return f.doFetch(ctx, repo, key, params)
	})

	if err == nil {
		f.cache.Set(flightKey, val)
	}

	return val, err
}

func withFetchChain(ctx context.Context, flightKey string) (context.Context, error) {
	chain := getFetchChain(ctx)
	for _, existing := range chain {
		if existing == flightKey {
			return nil, fmt.Errorf("Fetch: dependency cycle detected: %s -> %s", strings.Join(chain, " -> "), flightKey)
		}
	}

	updated := make([]string, 0, len(chain)+1)
	updated = append(updated, chain...)
	updated = append(updated, flightKey)
	return context.WithValue(ctx, fetchChainKey{}, updated), nil
}

func getFetchChain(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(fetchChainKey{})
	chain, ok := v.([]string)
	if !ok {
		return nil
	}
	return chain
}

func (f *Fetcher) doFetch(ctx context.Context, repo *github.Repository, key data.DependencyKey, params map[string]string) (any, error) {
	fetchImpl, ok := ResolveDataFetcher(key)
	if !ok {
		return nil, fmt.Errorf("unsupported dependency key: %s", key)
	}
	return fetchImpl.Fetch(ctx, repo, params, f)
}

func makeFlightKey(repo *github.Repository, scope data.FetchScope, key data.DependencyKey, params map[string]string) (string, error) {
	var prefix string
	switch scope {
	case data.ScopeOrg:
		owner := strings.ToLower(repo.GetOwner().GetLogin())
		if owner == "" {
			return "", fmt.Errorf("Fetch: repo owner login is required for org-scoped dependency: %s", key)
		}
		prefix = owner
	case data.ScopeRepo:
		repoID := repo.GetFullName()
		if repoID == "" {
			owner := strings.ToLower(repo.GetOwner().GetLogin())
			name := strings.ToLower(repo.GetName())
			if owner == "" || name == "" {
				return "", fmt.Errorf("Fetch: repo owner/name is required for repo-scoped dependency: %s", key)
			}
			repoID = owner + "/" + name
		}
		prefix = strings.ToLower(repoID)
	default:
		return "", fmt.Errorf("Fetch: unknown fetch scope %q for dependency: %s", scope, key)
	}

	return prefix + ":" + string(key) + ":" + stableParamsKey(params), nil
}

func stableParamsKey(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}
