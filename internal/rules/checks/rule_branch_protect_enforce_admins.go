package checks

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"strings"

	"github.com/google/go-github/v81/github"
)

type BranchProtectEnforceAdmins struct{}

func (r *BranchProtectEnforceAdmins) ID() string {
	return "branch-protect-enforce-admins"
}

func (r *BranchProtectEnforceAdmins) Title() string {
	return "Enforce Admins on Protected Branches"
}

func (r *BranchProtectEnforceAdmins) Description() string {
	return "Verifies that 'Enforce admins' is enabled for all classic branch protection rules. This ensures that branch protection settings apply to repository administrators. If no classic branch protection rules exist, this check passes."
}

func (r *BranchProtectEnforceAdmins) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoClassicBranchProtections,
	}, nil
}

func (r *BranchProtectEnforceAdmins) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoClassicBranchProtections)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("missing dependency: %s", data.DepRepoClassicBranchProtections)), nil
	}

	protections, ok := val.(*models.ClassicBranchProtections)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("unexpected type for %s: %T", data.DepRepoClassicBranchProtections, val)), nil
	}

	if len(protections.Protections) == 0 {
		return rules.PassResultWithMessage(repo, r.ID(), "No classic branch protection rules found, so there are no admin enforcement settings to check"), nil
	}

	var failedPatterns []string
	for _, p := range protections.Protections {
		if !p.IsAdminEnforced {
			failedPatterns = append(failedPatterns, p.Pattern)
		}
	}

	if len(failedPatterns) > 0 {
		return rules.FailResult(repo, r.ID(), fmt.Sprintf("Enforce admins is disabled for classic branch protection rules: %s", strings.Join(failedPatterns, ", "))), nil
	}

	return rules.PassResultWithMessage(repo, r.ID(), "Enforce admins is enabled for all classic branch protection rules"), nil
}

func init() {
	rules.Register(&BranchProtectEnforceAdmins{})
}
