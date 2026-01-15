package providers

import (
	"context"
	"errors"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v81/github"
)

// reposScannedFetcher returns the list of repositories discovered for the current scan.
//
// This is an org-scoped dependency that is injected by the engine (via SetScannedRepos)
// rather than fetched from GitHub. It enables other org-scoped fetchers to derive
// baselines or conventions from the scanned repository set without additional API calls.
type reposScannedFetcher struct{}

func (r *reposScannedFetcher) Key() data.DependencyKey { return data.DepReposScanned }

func (r *reposScannedFetcher) Scope() data.FetchScope { return data.ScopeOrg }

func (r *reposScannedFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	repos := f.ScannedRepos()
	if repos == nil {
		return nil, errors.New("scanned repos not available: SetScannedRepos was not called")
	}
	return repos, nil
}

func init() {
	fetcher.RegisterDataFetcher(&reposScannedFetcher{})
}
