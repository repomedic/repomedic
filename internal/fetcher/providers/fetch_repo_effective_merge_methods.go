package providers

import (
	"context"
	"encoding/json"
	"strings"

	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v66/github"
)

// repoEffectiveMergeMethodsFetcher computes the effective allowed merge methods
// for a repository's default branch, considering:
//   - Repository settings (allow_merge_commit, allow_squash_merge, allow_rebase_merge)
//   - Rulesets (including org rulesets) that apply to the default branch
//
// Rulesets can constrain methods via:
//   - required_linear_history: removes merge commit option
//   - merge_queue: may constrain to a single method
type repoEffectiveMergeMethodsFetcher struct{}

func (r *repoEffectiveMergeMethodsFetcher) Key() data.DependencyKey {
	return data.DepRepoEffectiveMergeMethods
}

func (r *repoEffectiveMergeMethodsFetcher) Scope() data.FetchScope {
	return data.ScopeRepo
}

func (r *repoEffectiveMergeMethodsFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	// Get repo metadata for merge method settings.
	metaResult, err := f.Fetch(ctx, repo, data.DepRepoMetadata, nil)
	if err != nil {
		return nil, err
	}

	repoMeta, ok := metaResult.(*github.Repository)
	if !ok || repoMeta == nil {
		return models.MergeMethodMask(0), nil
	}

	// Start with repo settings.
	allowed := maskFromRepoBools(repoMeta)

	// If no methods allowed at repo level, return early.
	if allowed == 0 {
		return allowed, nil
	}

	// Get rulesets to check for additional constraints.
	rulesetsResult, err := f.Fetch(ctx, repo, data.DepRepoAllRulesets, nil)
	if err != nil {
		// If rulesets fetch fails (e.g., 403), return repo settings only.
		return allowed, nil
	}

	// Handle nil (no rulesets configured).
	if rulesetsResult == nil {
		return allowed, nil
	}

	rulesets, ok := rulesetsResult.([]*github.Ruleset)
	if !ok {
		// Invalid type - return repo settings only.
		return allowed, nil
	}

	if len(rulesets) == 0 {
		return allowed, nil
	}

	// Determine target ref for the default branch.
	defaultBranch := repoMeta.GetDefaultBranch()
	if defaultBranch == "" {
		defaultBranch = "main" // Fallback.
	}
	targetRef := "refs/heads/" + defaultBranch

	// Apply constraints from applicable rulesets.
	allowed = applyRulesetConstraints(rulesets, targetRef, allowed)

	return allowed, nil
}

// applyRulesetConstraints applies merge method constraints from rulesets that
// target the given ref and are actively enforced.
func applyRulesetConstraints(rulesets []*github.Ruleset, targetRef string, allowed models.MergeMethodMask) models.MergeMethodMask {
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

		// Apply rule constraints.
		allowed = applyRulesConstraints(rs.Rules, allowed)

		// Early exit if no methods left.
		if allowed == 0 {
			return 0
		}
	}

	return allowed
}

// applyRulesConstraints applies constraints from individual rules within a ruleset.
func applyRulesConstraints(rules []*github.RepositoryRule, allowed models.MergeMethodMask) models.MergeMethodMask {
	for _, rule := range rules {
		if rule == nil {
			continue
		}

		switch rule.Type {
		case "required_linear_history":
			// Linear history prohibits merge commits.
			allowed = allowed &^ models.MergeMethodMerge

		case "merge_queue":
			// Merge queue may specify a single method.
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
		}
	}

	return allowed
}

func init() {
	fetcher.RegisterDataFetcher(&repoEffectiveMergeMethodsFetcher{})
}
