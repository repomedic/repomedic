package providers

import (
	"context"
	"fmt"
	"strings"

	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"
	gh "repomedic/internal/github"

	"github.com/google/go-github/v66/github"
)

// Keep GitHub calls bounded.
//
// This limit applies to the number of distinct protection scopes checked (e.g.
// wildcard patterns), not the number of branches in the repository.
const protectedBranchesDeletionStatusLimit = 50

type protectedBranchesDeletionStatusFetcher struct{}

func (p *protectedBranchesDeletionStatusFetcher) Key() data.DependencyKey {
	return data.DepRepoProtectedBranchesDeletionStatus
}

func (p *protectedBranchesDeletionStatusFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	owner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()

	out := &models.ProtectedBranchesDeletionStatus{
		Branches:  make([]models.ProtectedBranchDeletionStatus, 0, 16),
		Truncated: false,
		Limit:     protectedBranchesDeletionStatusLimit,
	}

	// Collect raw scopes from both sources, then deduplicate.
	var rawScopes []models.ProtectedBranchDeletionStatus

	// Classic branch protection rules (wildcard patterns) via GraphQL.
	classic, truncated, err := fetchClassicBranchProtectionDeletionScopes(ctx, owner, repoName, f, protectedBranchesDeletionStatusLimit)
	if err != nil {
		return nil, err
	}
	rawScopes = append(rawScopes, classic...)
	if truncated {
		out.Truncated = true
		out.Branches = deduplicateScopes(rawScopes)
		return out, nil
	}

	remaining := protectedBranchesDeletionStatusLimit - len(rawScopes)
	if remaining <= 0 {
		out.Truncated = true
		out.Branches = deduplicateScopes(rawScopes)
		return out, nil
	}

	// Ruleset scopes (including org/inherited): evaluate patterns directly.
	rulesetScopes, truncated, err := fetchRulesetDeletionScopes(ctx, owner, repoName, f, remaining)
	if err != nil {
		return nil, err
	}
	rawScopes = append(rawScopes, rulesetScopes...)
	if truncated {
		out.Truncated = true
	}

	out.Branches = deduplicateScopes(rawScopes)
	return out, nil
}

// normalizePattern converts patterns from different sources to a comparable form.
// Classic patterns: "main", "release/*"
// Ruleset patterns: "refs/heads/main", "refs/heads/release/*", "~DEFAULT_BRANCH"
func normalizePattern(pattern string) string {
	return strings.TrimPrefix(pattern, "refs/heads/")
}

// deduplicateScopes merges scopes with the same normalized pattern.
// If ANY source blocks deletion for a pattern, the merged scope blocks deletion.
// Sources are combined into a comma-separated list.
func deduplicateScopes(scopes []models.ProtectedBranchDeletionStatus) []models.ProtectedBranchDeletionStatus {
	type entry struct {
		normalizedPattern string
		displayName       string
		deletionBlocked   bool
		sources           []string
		details           []string
	}

	seen := make(map[string]*entry)
	order := make([]string, 0, len(scopes))

	for _, s := range scopes {
		norm := normalizePattern(s.Name)
		if e, ok := seen[norm]; ok {
			// Merge: deletion is blocked if ANY source blocks it.
			e.deletionBlocked = e.deletionBlocked || s.DeletionBlocked
			e.sources = append(e.sources, s.Source)
			if s.Detail != "" {
				e.details = append(e.details, s.Detail)
			}
		} else {
			seen[norm] = &entry{
				normalizedPattern: norm,
				displayName:       norm, // Use normalized pattern as display name.
				deletionBlocked:   s.DeletionBlocked,
				sources:           []string{s.Source},
				details:           []string{s.Detail},
			}
			order = append(order, norm)
		}
	}

	result := make([]models.ProtectedBranchDeletionStatus, 0, len(order))
	for _, norm := range order {
		e := seen[norm]
		result = append(result, models.ProtectedBranchDeletionStatus{
			Name:            e.displayName,
			DeletionBlocked: e.deletionBlocked,
			Source:          strings.Join(e.sources, ", "),
			Detail:          strings.Join(e.details, "; "),
		})
	}
	return result
}

type graphQLBranchProtectionRulesData struct {
	Repository struct {
		BranchProtectionRules struct {
			Nodes []struct {
				Pattern         string `json:"pattern"`
				AllowsDeletions bool   `json:"allowsDeletions"`
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool    `json:"hasNextPage"`
				EndCursor   *string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"branchProtectionRules"`
	} `json:"repository"`
}

func fetchClassicBranchProtectionDeletionScopes(ctx context.Context, owner, repo string, f *fetcher.Fetcher, limit int) ([]models.ProtectedBranchDeletionStatus, bool, error) {
	if limit <= 0 {
		return nil, true, nil
	}

	query := `query($owner:String!, $name:String!, $after:String) {
  repository(owner:$owner, name:$name) {
    branchProtectionRules(first:100, after:$after) {
      nodes { pattern allowsDeletions }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

	items := make([]models.ProtectedBranchDeletionStatus, 0, 8)
	seen := map[string]struct{}{}
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

		resp, httpResp, err := gh.DoGraphQL[graphQLBranchProtectionRulesData](ctx, f.Client(), greq)
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
			key := "classic:" + pattern
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			items = append(items, models.ProtectedBranchDeletionStatus{
				Name:            pattern,
				DeletionBlocked: !n.AllowsDeletions,
				Source:          "classic-branch-protection",
				Detail:          "allowsDeletions=" + fmt.Sprintf("%t", n.AllowsDeletions),
			})
			if len(items) >= limit {
				return items[:limit], true, nil
			}
		}

		if !resp.Data.Repository.BranchProtectionRules.PageInfo.HasNextPage {
			break
		}
		after = resp.Data.Repository.BranchProtectionRules.PageInfo.EndCursor
		if after == nil {
			break
		}
	}

	return items, false, nil
}

func fetchRulesetDeletionScopes(ctx context.Context, owner, repo string, f *fetcher.Fetcher, limit int) ([]models.ProtectedBranchDeletionStatus, bool, error) {
	if limit <= 0 {
		return nil, true, nil
	}

	if err := f.Budget().Acquire(ctx, 1); err != nil {
		return nil, false, err
	}
	// includesParents=true to cover org-level rulesets applying to this repo.
	rulesets, resp, err := f.Client().Client.Repositories.GetAllRulesets(ctx, owner, repo, true)
	if resp != nil {
		f.Budget().UpdateFromResponse(resp.Response)
	}
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, false, nil
		}
		return nil, false, err
	}

	// The list endpoint may omit conditions/rules, so fetch each ruleset for details.
	items := make([]models.ProtectedBranchDeletionStatus, 0, 8)

	for _, rs := range rulesets {
		if rs == nil || rs.ID == nil {
			continue
		}
		// Only branch-targeting rulesets; skip tags/push.
		if rs.Target != nil && !strings.EqualFold(*rs.Target, "branch") {
			continue
		}
		// Only active rulesets are enforced. "evaluate" is non-enforcing.
		if !strings.EqualFold(strings.TrimSpace(rs.Enforcement), "active") {
			continue
		}

		if len(items) >= limit {
			return items[:limit], true, nil
		}

		if err := f.Budget().Acquire(ctx, 1); err != nil {
			return nil, false, err
		}
		detail, rsResp, rsErr := f.Client().Client.Repositories.GetRuleset(ctx, owner, repo, *rs.ID, true)
		if rsResp != nil {
			f.Budget().UpdateFromResponse(rsResp.Response)
		}
		if rsErr != nil {
			// If we can't read rulesets due to perms, surface ERROR.
			return nil, false, rsErr
		}
		if detail == nil {
			continue
		}

		deletionBlocked := rulesetBlocksDeletion(detail)

		includes := []string{}
		if detail.Conditions != nil && detail.Conditions.RefName != nil {
			includes = append(includes, detail.Conditions.RefName.Include...)
		}

		rulesetSource := fmt.Sprintf("ruleset:%s", detail.Name)

		// If there are no include patterns, count this as one broad scope.
		if len(includes) == 0 {
			items = append(items, models.ProtectedBranchDeletionStatus{
				Name:            "<all-refs>",
				DeletionBlocked: deletionBlocked,
				Source:          rulesetSource,
				Detail:          fmt.Sprintf("id=%d", detail.GetID()),
			})
			continue
		}

		for _, pat := range includes {
			if len(items) >= limit {
				return items[:limit], true, nil
			}

			pattern := strings.TrimSpace(pat)
			if pattern == "" {
				continue
			}

			items = append(items, models.ProtectedBranchDeletionStatus{
				Name:            pattern,
				DeletionBlocked: deletionBlocked,
				Source:          rulesetSource,
				Detail:          fmt.Sprintf("id=%d", detail.GetID()),
			})
		}
	}

	return items, false, nil
}

func rulesetBlocksDeletion(rs *github.Ruleset) bool {
	if rs == nil {
		return false
	}
	for _, rule := range rs.Rules {
		if rule == nil {
			continue
		}
		if strings.EqualFold(rule.Type, "deletion") {
			return true
		}
	}
	return false
}

func init() {
	fetcher.RegisterDataFetcher(&protectedBranchesDeletionStatusFetcher{})
}
