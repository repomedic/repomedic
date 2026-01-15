package providers

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v81/github"
)

type defaultBranchReadmeFetcher struct{}

func (d *defaultBranchReadmeFetcher) Key() data.DependencyKey {
	return data.DepRepoDefaultBranchReadme
}

func (d *defaultBranchReadmeFetcher) Scope() data.FetchScope {
	return data.ScopeRepo
}

func (d *defaultBranchReadmeFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
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

	owner := repo.GetOwner().GetLogin()
	name := repo.GetName()

	presence := &models.ReadmePresence{}

	if err := f.Budget().Acquire(ctx, 1); err != nil {
		return nil, err
	}

	content, resp, err := f.Client().Client.Repositories.GetReadme(ctx, owner, name, &github.RepositoryContentGetOptions{Ref: branch})
	if resp != nil {
		f.Budget().UpdateFromResponse(resp.Response)
	}
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return presence, nil
		}
		return nil, err
	}

	presence.Found = true
	if content != nil {
		presence.Path = content.GetPath()
	}

	return presence, nil
}

func init() {
	fetcher.RegisterDataFetcher(&defaultBranchReadmeFetcher{})
}
