package providers

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v66/github"
)

// Keep GitHub calls bounded.
const orgRulesetsLimit = 100

// orgMergeBaselineFetcher derives a merge-method baseline from organization rulesets.
//
// Algorithm:
//  1. Determine the target ref as the most common default branch among scanned repos.
//  2. Fetch org rulesets via GitHub API (bounded).
//  3. Apply only actively enforced rulesets targeting branches that match the ref.
//  4. Intersect allowed merge methods; apply linear-history to remove merge commits.
//  5. Return set (with mask), conflict (if mask becomes 0), or none (no applicable rulesets).
type orgMergeBaselineFetcher struct{}

func (o *orgMergeBaselineFetcher) Key() data.DependencyKey {
	return data.DepOrgMergeBaseline
}

func (o *orgMergeBaselineFetcher) Scope() data.FetchScope {
	return data.ScopeOrg
}

func (o *orgMergeBaselineFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	// Get scanned repos to determine the most common default branch.
	scannedResult, err := f.Fetch(ctx, repo, data.DepReposScanned, nil)
	if err != nil {
		return nil, err
	}
	scannedRepos, ok := scannedResult.([]*github.Repository)
	if !ok {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceOrganizationRuleset,
			Evidence: []string{"invalid scanned repos type"},
		}, nil
	}

	if len(scannedRepos) == 0 {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceOrganizationRuleset,
			Evidence: []string{"no scanned repos available"},
		}, nil
	}

	// Determine target ref from most common default branch.
	targetRef := determineTargetRef(scannedRepos)

	// Fetch org rulesets.
	owner := repo.GetOwner().GetLogin()
	if owner == "" {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceOrganizationRuleset,
			Evidence: []string{"owner login not available"},
		}, nil
	}

	if err := f.Budget().Acquire(ctx, 1); err != nil {
		return nil, err
	}

	rulesets, resp, err := f.Client().Client.Organizations.GetAllOrganizationRulesets(ctx, owner)
	if resp != nil {
		f.Budget().UpdateFromResponse(resp.Response)
	}
	if err != nil {
		// Handle 404 (no rulesets) or 403 (no permission).
		if resp != nil && (resp.StatusCode == 404 || resp.StatusCode == 403) {
			return &models.MergeBaseline{
				State:    models.BaselineStateNone,
				Source:   models.BaselineSourceOrganizationRuleset,
				Evidence: []string{"org rulesets not accessible or not configured"},
			}, nil
		}
		return nil, err
	}

	if len(rulesets) == 0 {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceOrganizationRuleset,
			Evidence: []string{"no org rulesets configured"},
		}, nil
	}

	// Apply limit.
	if len(rulesets) > orgRulesetsLimit {
		rulesets = rulesets[:orgRulesetsLimit]
	}

	// Filter to actively enforced rulesets targeting branches that match targetRef.
	applicableRulesets := filterApplicableOrgRulesets(rulesets, targetRef)

	if len(applicableRulesets) == 0 {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceOrganizationRuleset,
			Evidence: []string{"no applicable org rulesets for " + targetRef},
		}, nil
	}

	// Derive allowed merge methods from applicable rulesets.
	return deriveBaselineFromRulesets(applicableRulesets, targetRef)
}

// determineTargetRef finds the most common default branch among scanned repos
// and returns it as a refs/heads/ ref. Ties are broken lexicographically ascending.
func determineTargetRef(repos []*github.Repository) string {
	branchCounts := make(map[string]int)
	for _, r := range repos {
		if r == nil {
			continue
		}
		branch := r.GetDefaultBranch()
		if branch != "" {
			branchCounts[branch]++
		}
	}

	if len(branchCounts) == 0 {
		return "refs/heads/main" // Fallback.
	}

	// Find max count.
	maxCount := 0
	for _, count := range branchCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	// Collect all branches with max count for deterministic tie-breaking.
	var winners []string
	for branch, count := range branchCounts {
		if count == maxCount {
			winners = append(winners, branch)
		}
	}

	// Sort lexicographically ascending for deterministic tie-break.
	sort.Strings(winners)

	return "refs/heads/" + winners[0]
}

// filterApplicableOrgRulesets returns rulesets that are actively enforced
// and target branches matching the given ref.
func filterApplicableOrgRulesets(rulesets []*github.Ruleset, targetRef string) []*github.Ruleset {
	var result []*github.Ruleset

	for _, rs := range rulesets {
		if rs == nil {
			continue
		}

		// Only actively enforced rulesets.
		if strings.ToLower(rs.Enforcement) != "active" {
			continue
		}

		// Only branch-targeting rulesets.
		if rs.GetTarget() != "" && rs.GetTarget() != "branch" {
			continue
		}

		// Check ref conditions.
		if !rulesetMatchesRef(rs, targetRef) {
			continue
		}

		result = append(result, rs)
	}

	return result
}

// rulesetMatchesRef checks if a ruleset's ref conditions match the target ref.
func rulesetMatchesRef(rs *github.Ruleset, targetRef string) bool {
	cond := rs.GetConditions()
	if cond == nil {
		// No conditions means applies to all branches.
		return true
	}

	refCond := cond.GetRefName()
	if refCond == nil {
		// No ref conditions means applies to all branches.
		return true
	}

	// Check exclusions first.
	for _, pattern := range refCond.Exclude {
		if refMatchesPattern(targetRef, pattern) {
			return false
		}
	}

	// If no inclusions specified, it matches all (after exclusions).
	if len(refCond.Include) == 0 {
		return true
	}

	// Check if any inclusion pattern matches.
	for _, pattern := range refCond.Include {
		if refMatchesPattern(targetRef, pattern) {
			return true
		}
	}

	return false
}

// refMatchesPattern checks if a ref matches a ruleset pattern.
// Patterns can be:
// - "~DEFAULT_BRANCH" → matches any default branch
// - "~ALL" → matches all refs
// - "refs/heads/*" → wildcard matching
// - "refs/heads/main" → exact match
func refMatchesPattern(ref, pattern string) bool {
	switch pattern {
	case "~DEFAULT_BRANCH":
		// Default branch pattern matches any default branch ref.
		return strings.HasPrefix(ref, "refs/heads/")
	case "~ALL":
		return true
	}

	// Wildcard matching.
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(ref, prefix)
	}

	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "**")
		return strings.HasPrefix(ref, prefix)
	}

	// Exact match.
	return ref == pattern
}

// deriveBaselineFromRulesets computes the allowed merge methods from applicable rulesets.
func deriveBaselineFromRulesets(rulesets []*github.Ruleset, targetRef string) (*models.MergeBaseline, error) {
	// Start with all methods allowed.
	allowed := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	var evidence []string
	hasConstraints := false

	for _, rs := range rulesets {
		if rs == nil || rs.Rules == nil {
			continue
		}

		for _, rule := range rs.Rules {
			if rule == nil {
				continue
			}

			switch rule.Type {
			case "required_linear_history":
				// Linear history prohibits merge commits.
				if allowed.Has(models.MergeMethodMerge) {
					allowed = allowed &^ models.MergeMethodMerge
					evidence = append(evidence, rs.Name+": required_linear_history (removes merge)")
					hasConstraints = true
				}

			case "merge_queue":
				// Merge queue can specify a single merge method.
				params := rule.GetParameters()
				if params == nil {
					continue
				}

				var mqParams github.MergeQueueRuleParameters
				if err := json.Unmarshal(params, &mqParams); err != nil {
					continue
				}

				method := strings.ToUpper(mqParams.MergeMethod)
				if method == "" {
					continue
				}

				var methodMask models.MergeMethodMask
				switch method {
				case "MERGE":
					methodMask = models.MergeMethodMerge
				case "SQUASH":
					methodMask = models.MergeMethodSquash
				case "REBASE":
					methodMask = models.MergeMethodRebase
				default:
					continue
				}

				// Merge queue constrains to a single method.
				allowed = allowed.Intersect(methodMask)
				evidence = append(evidence, rs.Name+": merge_queue ("+strings.ToLower(method)+" only)")
				hasConstraints = true
			}
		}
	}

	if !hasConstraints {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceOrganizationRuleset,
			Evidence: []string{"no merge method constraints in applicable org rulesets for " + targetRef},
		}, nil
	}

	if allowed == 0 {
		return &models.MergeBaseline{
			State:    models.BaselineStateConflict,
			Source:   models.BaselineSourceOrganizationRuleset,
			Allowed:  0,
			Evidence: evidence,
		}, nil
	}

	return &models.MergeBaseline{
		State:    models.BaselineStateSet,
		Source:   models.BaselineSourceOrganizationRuleset,
		Allowed:  allowed,
		Evidence: evidence,
	}, nil
}

func init() {
	fetcher.RegisterDataFetcher(&orgMergeBaselineFetcher{})
}
