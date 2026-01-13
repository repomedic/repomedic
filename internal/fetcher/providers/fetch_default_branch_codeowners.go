package providers

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v66/github"
)

type defaultBranchCodeownersFetcher struct{}

func (d *defaultBranchCodeownersFetcher) Key() data.DependencyKey {
	return data.DepRepoDefaultBranchCodeowners
}

func (d *defaultBranchCodeownersFetcher) Scope() data.FetchScope {
	return data.ScopeRepo
}

func (d *defaultBranchCodeownersFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
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

	presence := &models.CodeownersPresence{}

	owner := repo.GetOwner().GetLogin()
	name := repo.GetName()

	check := func(path string) (exists bool, err error) {
		if err := f.Budget().Acquire(ctx, 1); err != nil {
			return false, err
		}

		_, _, resp, err := f.Client().Client.Repositories.GetContents(ctx, owner, name, path, &github.RepositoryContentGetOptions{Ref: branch})
		if resp != nil {
			f.Budget().UpdateFromResponse(resp.Response)
		}
		if err != nil {
			if resp != nil && resp.StatusCode == 404 {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	exists, err := check("CODEOWNERS")
	if err != nil {
		return nil, err
	}
	presence.Root = exists

	exists, err = check(".github/CODEOWNERS")
	if err != nil {
		return nil, err
	}
	presence.GitHub = exists

	return presence, nil
}

func init() {
	fetcher.RegisterDataFetcher(&defaultBranchCodeownersFetcher{})
}
