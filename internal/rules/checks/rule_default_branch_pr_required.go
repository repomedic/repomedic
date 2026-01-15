package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v81/github"
)

type DefaultBranchPRRequiredRule struct{}

func (r *DefaultBranchPRRequiredRule) ID() string {
	return "default-branch-pr-required"
}

func (r *DefaultBranchPRRequiredRule) Title() string {
	return "Default Branch Requires Pull Requests"
}

func (r *DefaultBranchPRRequiredRule) Description() string {
	return "Verifies that the repository's default branch requires changes to be merged via pull request.\n\n" +
		"This passes if the requirement is enforced either by classic branch protection or by GitHub rulesets (including inherited org rulesets). " +
		"Bypass actors may still exist; this rule only checks whether a PR requirement is present for the default branch."
}

func (r *DefaultBranchPRRequiredRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoMetadata,
		data.DepRepoDefaultBranchClassicProtection,
		data.DepRepoDefaultBranchEffectiveRules,
	}, nil
}

func (r *DefaultBranchPRRequiredRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
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

	classicPRRequired, errMsg := classicProtectionRequiresPR(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	rulesetPRRequired, errMsg := effectiveRulesRequirePR(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	if classicPRRequired || rulesetPRRequired {
		return rules.PassResult(repo, r.ID()), nil
	}

	return rules.FailResult(repo, r.ID(), "Default branch does not require pull requests to merge"), nil
}

func classicProtectionRequiresPR(dc data.DataContext) (bool, string) {
	val, ok := dc.Get(data.DepRepoDefaultBranchClassicProtection)
	if !ok {
		return false, "Dependency missing"
	}

	if val == nil {
		return false, ""
	}

	protection, ok := val.(*github.Protection)
	if !ok {
		return false, "Invalid dependency type"
	}

	return protection.RequiredPullRequestReviews != nil, ""
}

func effectiveRulesRequirePR(dc data.DataContext) (bool, string) {
	val, ok := dc.Get(data.DepRepoDefaultBranchEffectiveRules)
	if !ok {
		return false, "Dependency missing"
	}
	if val == nil {
		return false, ""
	}

	rules, ok := val.(*github.BranchRules)
	if !ok {
		return false, "Invalid dependency type"
	}

	// In v81, PullRequest is a slice; if non-empty, PRs are required
	if len(rules.PullRequest) > 0 {
		return true, ""
	}
	return false, ""
}

func init() {
	rules.Register(&DefaultBranchPRRequiredRule{})
}
