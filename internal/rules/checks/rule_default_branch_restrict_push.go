package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strings"

	"github.com/google/go-github/v66/github"
)

type DefaultBranchRestrictPushRule struct{}

func (r *DefaultBranchRestrictPushRule) ID() string {
	return "default-branch-restrict-push"
}

func (r *DefaultBranchRestrictPushRule) Title() string {
	return "Default Branch Restricts Who Can Push"
}

func (r *DefaultBranchRestrictPushRule) Description() string {
	return "Verifies that the repository's default branch restricts who can push.\n\n" +
		"This passes if push restrictions are enabled either by classic branch protection " +
		"(Restrict who can push to matching branches) or by GitHub rulesets " +
		"(Restrict updates: Only allow users with bypass permission to update matching refs). " +
		"This ensures that only authorized actors (users, teams, apps) can push to the default branch."
}

func (r *DefaultBranchRestrictPushRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoMetadata,
		data.DepRepoDefaultBranchClassicProtection,
		data.DepRepoDefaultBranchEffectiveRules,
	}, nil
}

func (r *DefaultBranchRestrictPushRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
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

	classicRestrictsPush, errMsg := classicProtectionRestrictsPush(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	rulesetRestrictsPush, errMsg := effectiveRulesRestrictPush(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	if classicRestrictsPush || rulesetRestrictsPush {
		return rules.PassResult(repo, r.ID()), nil
	}

	return rules.FailResult(repo, r.ID(), "Default branch does not restrict who can push"), nil
}

func classicProtectionRestrictsPush(dc data.DataContext) (bool, string) {
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

	// If Restrictions is not nil, then "Restrict who can push" is enabled.
	return protection.Restrictions != nil, ""
}

func effectiveRulesRestrictPush(dc data.DataContext) (bool, string) {
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

	for _, rule := range rules {
		if rule == nil {
			continue
		}
		// "update" rule type restricts updates to the branch (pushes).
		if strings.EqualFold(rule.Type, "update") {
			return true, ""
		}
	}
	return false, ""
}

func init() {
	rules.Register(&DefaultBranchRestrictPushRule{})
}
