package providers

import (
	"context"
	"strings"

	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v81/github"
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

	rulesets, ok := rulesetsResult.([]*github.RepositoryRuleset)
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
func applyRulesetConstraints(rulesets []*github.RepositoryRuleset, targetRef string, allowed models.MergeMethodMask) models.MergeMethodMask {
	for _, rs := range rulesets {
		if rs == nil {
			continue
		}

		// Only actively enforced rulesets.
		if rs.Enforcement != github.RulesetEnforcementActive {
			continue
		}

		// Only branch-targeting rulesets.
		target := rs.GetTarget()
		if target != nil && *target != github.RulesetTargetBranch {
			continue
		}

		// Check ref conditions.
		if !rulesetMatchesRef(rs, targetRef) {
			continue
		}

		// Apply rule constraints.
		allowed = applyRulesConstraints(rs.GetRules(), allowed)

		// Early exit if no methods left.
		if allowed == 0 {
			return 0
		}
	}

	return allowed
}

// applyRulesConstraints applies constraints from structured rules within a ruleset.
func applyRulesConstraints(rules *github.RepositoryRulesetRules, allowed models.MergeMethodMask) models.MergeMethodMask {
	if rules == nil {
		return allowed
	}

	// Check required_linear_history rule.
	if rules.RequiredLinearHistory != nil {
		allowed = allowed &^ models.MergeMethodMerge
	}

	// Check merge_queue rule.
	if rules.MergeQueue != nil {
		method := string(rules.MergeQueue.MergeMethod)
		if method != "" {
			var methodMask models.MergeMethodMask
			switch strings.ToUpper(method) {
			case "MERGE":
				methodMask = models.MergeMethodMerge
			case "SQUASH":
				methodMask = models.MergeMethodSquash
			case "REBASE":
				methodMask = models.MergeMethodRebase
			}

			if methodMask != 0 {
				allowed = allowed.Intersect(methodMask)
			}
		}
	}

	// Check pull_request rule for allowed merge methods.
	if rules.PullRequest != nil && len(rules.PullRequest.AllowedMergeMethods) > 0 {
		var methodMask models.MergeMethodMask
		for _, method := range rules.PullRequest.AllowedMergeMethods {
			switch method {
			case github.PullRequestMergeMethodMerge:
				methodMask |= models.MergeMethodMerge
			case github.PullRequestMergeMethodSquash:
				methodMask |= models.MergeMethodSquash
			case github.PullRequestMergeMethodRebase:
				methodMask |= models.MergeMethodRebase
			}
		}

		if methodMask != 0 {
			allowed = allowed.Intersect(methodMask)
		}
	}

	return allowed
}

func init() {
	fetcher.RegisterDataFetcher(&repoEffectiveMergeMethodsFetcher{})
}
