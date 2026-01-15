package checks

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"sort"
	"strings"

	"github.com/google/go-github/v81/github"
)

type ProtectedBranchesBlockDeletionRule struct{}

func (r *ProtectedBranchesBlockDeletionRule) ID() string {
	return "protected-branches-block-deletion"
}

func (r *ProtectedBranchesBlockDeletionRule) Title() string {
	return "Protected Branches Block Deletion"
}

func (r *ProtectedBranchesBlockDeletionRule) Description() string {
	return "Verifies that deletion is blocked for all protected branches.\n\n" +
		"This rule checks that any branch covered by classic branch protection or GitHub rulesets " +
		"cannot be deleted. This prevents accidental loss of important history and ensures that " +
		"branch protection policies remain in effect."
}

func (r *ProtectedBranchesBlockDeletionRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoProtectedBranchesDeletionStatus,
	}, nil
}

func (r *ProtectedBranchesBlockDeletionRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoProtectedBranchesDeletionStatus)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Dependency missing"), nil
	}
	if val == nil {
		return rules.ErrorResult(repo, r.ID(), "Dependency is nil"), nil
	}

	report, ok := val.(*models.ProtectedBranchesDeletionStatus)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("Invalid dependency type: %T", val)), nil
	}

	if report.Truncated {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("Protected scope scan truncated at limit=%d; cannot safely determine deletion policy for all protected scopes", report.Limit)), nil
	}

	if len(report.Branches) == 0 {
		return rules.PassResultWithMessage(repo, r.ID(), "No protected branches detected"), nil
	}

	bad := make([]string, 0, 4)
	for _, b := range report.Branches {
		if strings.TrimSpace(b.Name) == "" {
			continue
		}
		if !b.DeletionBlocked {
			bad = append(bad, b.Name)
		}
	}

	if len(bad) == 0 {
		return rules.PassResultWithMessage(repo, r.ID(), "Deletion blocked for all protected branches"), nil
	}

	sort.Strings(bad)

	shown := bad
	if len(shown) > 10 {
		shown = shown[:10]
	}

	msg := fmt.Sprintf("Deletion allowed on protected branches: %s", strings.Join(shown, ", "))
	if len(shown) != len(bad) {
		msg += fmt.Sprintf(" (showing %d of %d)", len(shown), len(bad))
	}
	return rules.FailResult(repo, r.ID(), msg), nil
}

func init() {
	rules.Register(&ProtectedBranchesBlockDeletionRule{})
}
