package providers

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v81/github"
)

type repoMetadataFetcher struct{}

func (r *repoMetadataFetcher) Key() data.DependencyKey { return data.DepRepoMetadata }

func (r *repoMetadataFetcher) Scope() data.FetchScope { return data.ScopeRepo }

func (r *repoMetadataFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	if err := f.Budget().Acquire(ctx, 1); err != nil {
		return nil, err
	}

	result, resp, err := f.Client().Client.Repositories.Get(ctx, repo.GetOwner().GetLogin(), repo.GetName())
	if resp != nil {
		f.Budget().UpdateFromResponse(resp.Response)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func init() {
	fetcher.RegisterDataFetcher(&repoMetadataFetcher{})
}
