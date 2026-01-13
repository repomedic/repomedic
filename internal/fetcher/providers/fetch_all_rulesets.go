package providers

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v66/github"
)

// Keep GitHub calls bounded.
const allRulesetsLimit = 100

type allRulesetsFetcher struct{}

func (a *allRulesetsFetcher) Key() data.DependencyKey {
	return data.DepRepoAllRulesets
}

func (a *allRulesetsFetcher) Scope() data.FetchScope {
	return data.ScopeRepo
}

func (a *allRulesetsFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	owner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()

	if err := f.Budget().Acquire(ctx, 1); err != nil {
		return nil, err
	}

	// includesParents=true to cover org-level rulesets applying to this repo.
	rulesets, resp, err := f.Client().Client.Repositories.GetAllRulesets(ctx, owner, repoName, true)
	if resp != nil {
		f.Budget().UpdateFromResponse(resp.Response)
	}
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return []*github.Ruleset{}, nil
		}
		return nil, err
	}

	if rulesets == nil {
		return []*github.Ruleset{}, nil
	}

	// Apply limit to keep bounded.
	if len(rulesets) > allRulesetsLimit {
		return rulesets[:allRulesetsLimit], nil
	}

	return rulesets, nil
}

func init() {
	fetcher.RegisterDataFetcher(&allRulesetsFetcher{})
}
