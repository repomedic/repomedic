package providers

import (
	"context"

	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v66/github"
)

// mergeBaselineFetcher selects the final merge-method baseline from available sources.
//
// Priority order:
//  1. Organization ruleset baseline (if state is "set" or "conflict")
//  2. Convention baseline (if org baseline state is "none")
//
// This fetcher does not make any GitHub API calls; it composes results from
// DepOrgMergeBaseline and DepReposMergeConvention.
type mergeBaselineFetcher struct{}

func (m *mergeBaselineFetcher) Key() data.DependencyKey {
	return data.DepMergeBaseline
}

func (m *mergeBaselineFetcher) Scope() data.FetchScope {
	return data.ScopeOrg
}

func (m *mergeBaselineFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	// Fetch org baseline.
	orgResult, err := f.Fetch(ctx, repo, data.DepOrgMergeBaseline, nil)
	if err != nil {
		return nil, err
	}

	orgBaseline, ok := orgResult.(*models.MergeBaseline)
	if !ok || orgBaseline == nil {
		// Fall back to convention if org baseline is invalid.
		return fetchConventionBaseline(ctx, repo, f)
	}

	// If org baseline is set or conflict, use it.
	if orgBaseline.State == models.BaselineStateSet || orgBaseline.State == models.BaselineStateConflict {
		return orgBaseline, nil
	}

	// Org baseline is "none" - fall back to convention.
	return fetchConventionBaseline(ctx, repo, f)
}

// fetchConventionBaseline retrieves the convention baseline as a fallback.
func fetchConventionBaseline(ctx context.Context, repo *github.Repository, f *fetcher.Fetcher) (*models.MergeBaseline, error) {
	convResult, err := f.Fetch(ctx, repo, data.DepReposMergeConvention, nil)
	if err != nil {
		return nil, err
	}

	convBaseline, ok := convResult.(*models.MergeBaseline)
	if !ok || convBaseline == nil {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceConvention,
			Evidence: []string{"convention baseline unavailable"},
		}, nil
	}

	return convBaseline, nil
}

func init() {
	fetcher.RegisterDataFetcher(&mergeBaselineFetcher{})
}
