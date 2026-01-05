package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strings"

	"github.com/google/go-github/v66/github"
)

type DefaultBranchNoForcePushRule struct{}

func (r *DefaultBranchNoForcePushRule) ID() string {
	return "default-branch-no-force-push"
}

func (r *DefaultBranchNoForcePushRule) Title() string {
	return "Default Branch Blocks Force Pushes"
}

func (r *DefaultBranchNoForcePushRule) Description() string {
	return "Verifies that the repository's default branch blocks force pushes.\n\n" +
		"Force pushes can overwrite commit history, potentially destroying work and making it " +
		"difficult to track changes. This rule passes if force pushes are blocked either by " +
		"classic branch protection (AllowForcePushes disabled) or by GitHub rulesets " +
		"(non_fast_forward rule present, including inherited org rulesets). " +
		"Bypass actors may still exist; this rule only checks whether force push protection is present."
}

func (r *DefaultBranchNoForcePushRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoMetadata,
		data.DepRepoDefaultBranchClassicProtection,
		data.DepRepoDefaultBranchEffectiveRules,
	}, nil
}

func (r *DefaultBranchNoForcePushRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
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

	classicBlocksForcePush, errMsg := classicProtectionBlocksForcePush(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	rulesetBlocksForcePush, errMsg := effectiveRulesBlockForcePush(dc)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	if classicBlocksForcePush || rulesetBlocksForcePush {
		return rules.PassResult(repo, r.ID()), nil
	}

	return rules.FailResult(repo, r.ID(), "Default branch does not block force pushes"), nil
}

// classicProtectionBlocksForcePush checks if classic branch protection blocks force pushes.
// Returns true if protection exists and AllowForcePushes is either nil or explicitly disabled.
func classicProtectionBlocksForcePush(dc data.DataContext) (bool, string) {
	val, ok := dc.Get(data.DepRepoDefaultBranchClassicProtection)
	if !ok {
		return false, "Dependency missing"
	}

	if val == nil {
		// No classic protection configured
		return false, ""
	}

	protection, ok := val.(*github.Protection)
	if !ok {
		return false, "Invalid dependency type"
	}

	// Classic branch protection blocks force pushes by default.
	// Force pushes are only allowed if AllowForcePushes is explicitly set and Enabled is true.
	if protection.AllowForcePushes != nil && protection.AllowForcePushes.Enabled {
		return false, ""
	}

	// Protection exists and force pushes are not explicitly allowed
	return true, ""
}

// effectiveRulesBlockForcePush checks if any effective ruleset rule blocks force pushes.
// The "non_fast_forward" rule type blocks force pushes.
func effectiveRulesBlockForcePush(dc data.DataContext) (bool, string) {
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
		if strings.EqualFold(rule.Type, "non_fast_forward") {
			return true, ""
		}
	}
	return false, ""
}

func init() {
	rules.Register(&DefaultBranchNoForcePushRule{})
}
