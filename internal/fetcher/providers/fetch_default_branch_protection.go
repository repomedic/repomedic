package providers

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v66/github"
)

type defaultBranchProtectionFetcher struct{}

func (d *defaultBranchProtectionFetcher) Key() data.DependencyKey {
	return data.DepRepoDefaultBranchClassicProtection
}

func (d *defaultBranchProtectionFetcher) Scope() data.FetchScope {
	return data.ScopeRepo
}

func (d *defaultBranchProtectionFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	branch := repo.GetDefaultBranch()
	if branch == "" {
		val, err := f.Fetch(ctx, repo, data.DepRepoMetadata, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve default branch: %w", err)
		}
		r, ok := val.(*github.Repository)
		if !ok {
			return nil, fmt.Errorf("failed to resolve default branch: unexpected type %T for %s", val, data.DepRepoMetadata)
		}
		branch = r.GetDefaultBranch()
		if branch == "" {
			return nil, fmt.Errorf("failed to resolve default branch: empty default branch")
		}
	}

	if err := f.Budget().Acquire(ctx, 1); err != nil {
		return nil, err
	}

	result, resp, err := f.Client().Client.Repositories.GetBranchProtection(ctx, repo.GetOwner().GetLogin(), repo.GetName(), branch)
	if resp != nil {
		f.Budget().UpdateFromResponse(resp.Response)
	}
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func init() {
	fetcher.RegisterDataFetcher(&defaultBranchProtectionFetcher{})
}
