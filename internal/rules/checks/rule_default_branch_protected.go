package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v66/github"
)

type DefaultBranchProtectedRule struct{}

func (r *DefaultBranchProtectedRule) ID() string {
	return "default-branch-protected"
}

func (r *DefaultBranchProtectedRule) Title() string {
	return "Default Branch Is Protected"
}

func (r *DefaultBranchProtectedRule) Description() string {
	return "Verifies that the repository's default branch has some form of protection configured.\n\n" +
		"This passes if the default branch is protected by either classic branch protection rules or GitHub rulesets " +
		"(including inherited org-level rulesets). The rule fails if no protection of any kind is configured " +
		"for the default branch."
}

func (r *DefaultBranchProtectedRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoMetadata,
		data.DepRepoDefaultBranchClassicProtection,
		data.DepRepoDefaultBranchEffectiveRules,
	}, nil
}

func (r *DefaultBranchProtectedRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	defaultBranch := repo.GetDefaultBranch()
	if defaultBranch == "" {
		val, ok := dc.Get(data.DepRepoMetadata)
		if !ok {
			return rules.ErrorResult(repo, r.ID(), "Dependency missing"), nil
		}
		meta, ok := val.(*github.Repository)
		if !ok {
			return rules.ErrorResult(repo, r.ID(), "Invalid dependency type"), nil
		}
		defaultBranch = meta.GetDefaultBranch()
	}
	if defaultBranch == "" {
		return rules.ErrorResult(repo, r.ID(), "Default branch is unknown"), nil
	}

	classicProtected, errMsg := classicProtectionExists(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	rulesetProtected, errMsg := effectiveRulesExist(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	if classicProtected || rulesetProtected {
		return rules.PassResult(repo, r.ID()), nil
	}

	return rules.FailResult(repo, r.ID(), "Default branch is not protected"), nil
}

func classicProtectionExists(dc data.DataContext) (bool, string) {
	val, ok := dc.Get(data.DepRepoDefaultBranchClassicProtection)
	if !ok {
		return false, "Dependency missing"
	}

	if val == nil {
		return false, ""
	}

	// If val is not nil, classic protection exists
	return true, ""
}

func effectiveRulesExist(dc data.DataContext) (bool, string) {
	val, ok := dc.Get(data.DepRepoDefaultBranchEffectiveRules)
	if !ok {
		return false, "Dependency missing"
	}
	if val == nil {
		return false, ""
	}

	rules, ok := val.([]*github.RepositoryRule)
	if !ok {
		return false, "Invalid dependency type"
	}

	return len(rules) > 0, ""
}

func init() {
	rules.Register(&DefaultBranchProtectedRule{})
}
