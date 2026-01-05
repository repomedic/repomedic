package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"

	"github.com/google/go-github/v66/github"
)

// BranchProtectionExistsRule detects repositories that have no branch protection
// configured for any branch. This includes checking for classic branch
// protection rules and GitHub rulesets (including inherited org-level rulesets).
type BranchProtectionExistsRule struct{}

func (r *BranchProtectionExistsRule) ID() string {
	return "branch-protection-exists"
}

func (r *BranchProtectionExistsRule) Title() string {
	return "Repository Has Branch Protection"
}

func (r *BranchProtectionExistsRule) Description() string {
	return "Verifies that the repository has some form of branch protection configured.\n\n" +
		"This passes if any branch is protected by either classic branch protection rules or GitHub rulesets " +
		"(including inherited org-level rulesets). The rule fails if no protection of any kind is configured " +
		"for any branch in the repository."
}

func (r *BranchProtectionExistsRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoProtectedBranchesDeletionStatus,
	}, nil
}

func (r *BranchProtectionExistsRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoProtectedBranchesDeletionStatus)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Dependency missing"), nil
	}

	// nil means no protected branches data available
	if val == nil {
		return rules.FailResult(repo, r.ID(), "Repository has no branch protection rules or rulesets configured"), nil
	}

	status, ok := val.(*models.ProtectedBranchesDeletionStatus)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Invalid dependency type"), nil
	}

	if len(status.Branches) > 0 {
		return rules.PassResult(repo, r.ID()), nil
	}

	return rules.FailResult(repo, r.ID(), "Repository has no branch protection rules or rulesets configured"), nil
}

func init() {
	rules.Register(&BranchProtectionExistsRule{})
}
