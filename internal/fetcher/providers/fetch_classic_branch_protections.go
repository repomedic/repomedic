package providers

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"
	gh "repomedic/internal/github"
	"strings"

	"github.com/google/go-github/v66/github"
)

const classicBranchProtectionsLimit = 100

type classicBranchProtectionsFetcher struct{}

func (f *classicBranchProtectionsFetcher) Key() data.DependencyKey {
	return data.DepRepoClassicBranchProtections
}

func (f *classicBranchProtectionsFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, fch *fetcher.Fetcher) (any, error) {
	owner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()

	out := &models.ClassicBranchProtections{
		Protections: make([]models.ClassicBranchProtection, 0, 16),
		Truncated:   false,
		Limit:       classicBranchProtectionsLimit,
	}

	protections, truncated, err := fetchClassicBranchProtections(ctx, owner, repoName, fch, classicBranchProtectionsLimit)
	if err != nil {
		return nil, err
	}

	out.Protections = protections
	out.Truncated = truncated
	return out, nil
}

type graphQLClassicBranchProtectionData struct {
	Repository struct {
		BranchProtectionRules struct {
			Nodes []struct {
				Pattern         string `json:"pattern"`
				IsAdminEnforced bool   `json:"isAdminEnforced"`
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool    `json:"hasNextPage"`
				EndCursor   *string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"branchProtectionRules"`
	} `json:"repository"`
}

func fetchClassicBranchProtections(ctx context.Context, owner, repo string, f *fetcher.Fetcher, limit int) ([]models.ClassicBranchProtection, bool, error) {
	if limit <= 0 {
		return nil, true, nil
	}

	query := `query($owner:String!, $name:String!, $after:String) {
  repository(owner:$owner, name:$name) {
    branchProtectionRules(first:100, after:$after) {
      nodes { pattern isAdminEnforced }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

	items := make([]models.ClassicBranchProtection, 0, 8)
	var after *string
	for {
		if len(items) >= limit {
			return items[:limit], true, nil
		}

		if err := f.Budget().Acquire(ctx, 1); err != nil {
			return nil, false, err
		}

		greq := gh.GraphQLRequest{
			Query: query,
			Variables: map[string]interface{}{
				"owner": owner,
				"name":  repo,
				"after": after,
			},
		}

		//nolint:bodyclose // DoGraphQL closes the response body before returning.
		resp, httpResp, err := gh.DoGraphQL[graphQLClassicBranchProtectionData](ctx, f.Client(), greq)
		if httpResp != nil {
			f.Budget().UpdateFromResponse(httpResp)
		}
		if err != nil {
			return nil, false, err
		}

		for _, n := range resp.Data.Repository.BranchProtectionRules.Nodes {
			pattern := strings.TrimSpace(n.Pattern)
			if pattern == "" {
				continue
			}
			items = append(items, models.ClassicBranchProtection{
				Pattern:         pattern,
				IsAdminEnforced: n.IsAdminEnforced,
			})
		}

		if !resp.Data.Repository.BranchProtectionRules.PageInfo.HasNextPage {
			return items, false, nil
		}
		after = resp.Data.Repository.BranchProtectionRules.PageInfo.EndCursor
	}
}

func init() {
	fetcher.RegisterDataFetcher(&classicBranchProtectionsFetcher{})
}
