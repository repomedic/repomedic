package checks

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strconv"
	"strings"

	"github.com/google/go-github/v81/github"
)

type DefaultBranchPRReviewSettingsRule struct {
	minApprovingReviews              int
	enforceMinApprovingReviews       bool
	enforceLastPushApproval          bool
	enforceCodeOwnerReview           bool
	enforceDismissStaleReviewsOnPush bool
}

func (r *DefaultBranchPRReviewSettingsRule) ID() string {
	return "default-branch-pr-review-settings"
}

func (r *DefaultBranchPRReviewSettingsRule) Title() string {
	return "Default Branch PR Review Settings"
}

func (r *DefaultBranchPRReviewSettingsRule) Description() string {
	return "Verifies that the repository's default branch enforces pull request review settings.\n\n" +
		"By default, this rule enforces ALL of the following for the default branch:\n" +
		"- Require a minimum of 1 approving review\n" +
		"- Require approval of the most recent reviewable push\n" +
		"- Require review from Code Owners\n" +
		"- Dismiss stale pull request approvals when new commits are pushed\n\n" +
		"Each requirement can be toggled on/off via rule options (see --help). The rule passes if the enabled requirements are satisfied either by classic branch protection or by effective GitHub rulesets (including inherited org rulesets)."
}

func (r *DefaultBranchPRReviewSettingsRule) Options() []rules.Option {
	return []rules.Option{
		{
			Name:        "min_approving_reviews",
			Description: "Minimum required approving reviews when enforce_min_approving_reviews=true. Must be >= 1.",
			Default:     "1",
		},
		{
			Name:        "enforce_min_approving_reviews",
			Description: "If true, require at least min_approving_reviews approving reviews.",
			Default:     "true",
		},
		{
			Name:        "enforce_last_push_approval",
			Description: "If true, require approval of the most recent reviewable push (require_last_push_approval=true).",
			Default:     "true",
		},
		{
			Name:        "enforce_code_owner_review",
			Description: "If true, require review from Code Owners.",
			Default:     "true",
		},
		{
			Name:        "enforce_dismiss_stale_reviews_on_push",
			Description: "If true, dismiss stale approvals when new commits are pushed.",
			Default:     "true",
		},
	}
}

func (r *DefaultBranchPRReviewSettingsRule) Configure(opts map[string]string) error {
	// Defaults: everything on, min reviewers = 1.
	r.minApprovingReviews = 1
	r.enforceMinApprovingReviews = true
	r.enforceLastPushApproval = true
	r.enforceCodeOwnerReview = true
	r.enforceDismissStaleReviewsOnPush = true

	get := func(k string) (string, bool) {
		v, ok := opts[k]
		if !ok {
			return "", false
		}
		v = strings.TrimSpace(v)
		if v == "" {
			return "", false
		}
		return v, true
	}

	if v, ok := get("min_approving_reviews"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid value for min_approving_reviews: %s", v)
		}
		r.minApprovingReviews = n
	}

	if v, ok := get("enforce_min_approving_reviews"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid value for enforce_min_approving_reviews: %s", v)
		}
		r.enforceMinApprovingReviews = b
	}
	if v, ok := get("enforce_last_push_approval"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid value for enforce_last_push_approval: %s", v)
		}
		r.enforceLastPushApproval = b
	}
	if v, ok := get("enforce_code_owner_review"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid value for enforce_code_owner_review: %s", v)
		}
		r.enforceCodeOwnerReview = b
	}
	if v, ok := get("enforce_dismiss_stale_reviews_on_push"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid value for enforce_dismiss_stale_reviews_on_push: %s", v)
		}
		r.enforceDismissStaleReviewsOnPush = b
	}

	if r.enforceMinApprovingReviews && r.minApprovingReviews < 1 {
		return fmt.Errorf("min_approving_reviews must be >= 1 when enforce_min_approving_reviews=true")
	}

	return nil
}

func (r *DefaultBranchPRReviewSettingsRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoMetadata,
		data.DepRepoDefaultBranchClassicProtection,
		data.DepRepoDefaultBranchEffectiveRules,
	}, nil
}

func (r *DefaultBranchPRReviewSettingsRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	if !r.enforceMinApprovingReviews && !r.enforceLastPushApproval && !r.enforceCodeOwnerReview && !r.enforceDismissStaleReviewsOnPush {
		return rules.PassResultWithMessage(repo, r.ID(), "No checks enabled"), nil
	}

	classicOK, classicMsg, errMsg := classicPRReviewSettingsOK(dc, r)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	effectiveOK, effectiveMsg, errMsg := effectivePRReviewSettingsOK(dc, r)
	if errMsg != "" {
		return rules.ErrorResult(repo, r.ID(), errMsg), nil
	}

	if classicOK {
		return rules.PassResultWithMessage(repo, r.ID(), "Classic branch protection: "+classicMsg), nil
	}
	if effectiveOK {
		return rules.PassResultWithMessage(repo, r.ID(), "Effective rulesets: "+effectiveMsg), nil
	}

	msg := strings.TrimSpace(strings.Join([]string{
		"Required settings not satisfied.",
		"Classic branch protection: " + classicMsg + ".",
		"Effective rulesets: " + effectiveMsg + ".",
	}, " "))
	return rules.FailResult(repo, r.ID(), msg), nil
}

func classicPRReviewSettingsOK(dc data.DataContext, cfg *DefaultBranchPRReviewSettingsRule) (ok bool, detail string, errMsg string) {
	val, exists := dc.Get(data.DepRepoDefaultBranchClassicProtection)
	if !exists {
		return false, "unknown", "Dependency missing"
	}
	if val == nil {
		return false, "branch not protected (nil)", ""
	}

	protection, okType := val.(*github.Protection)
	if !okType {
		return false, "unknown", "Invalid dependency type"
	}

	rr := protection.RequiredPullRequestReviews
	if rr == nil {
		return false, "required_pull_request_reviews not configured", ""
	}

	parts := make([]string, 0, 4)
	okAll := true

	if cfg.enforceMinApprovingReviews {
		okMin := rr.RequiredApprovingReviewCount >= cfg.minApprovingReviews
		okAll = okAll && okMin
		parts = append(parts, fmt.Sprintf("min_approving_reviews=%d (need >=%d)", rr.RequiredApprovingReviewCount, cfg.minApprovingReviews))
	} else {
		parts = append(parts, "min_approving_reviews=skipped")
	}

	if cfg.enforceLastPushApproval {
		okLast := rr.RequireLastPushApproval
		okAll = okAll && okLast
		parts = append(parts, fmt.Sprintf("require_last_push_approval=%t (need true)", rr.RequireLastPushApproval))
	} else {
		parts = append(parts, "require_last_push_approval=skipped")
	}

	if cfg.enforceCodeOwnerReview {
		okOwner := rr.RequireCodeOwnerReviews
		okAll = okAll && okOwner
		parts = append(parts, fmt.Sprintf("require_code_owner_reviews=%t (need true)", rr.RequireCodeOwnerReviews))
	} else {
		parts = append(parts, "require_code_owner_reviews=skipped")
	}

	if cfg.enforceDismissStaleReviewsOnPush {
		okDismiss := rr.DismissStaleReviews
		okAll = okAll && okDismiss
		parts = append(parts, fmt.Sprintf("dismiss_stale_reviews_on_push=%t (need true)", rr.DismissStaleReviews))
	} else {
		parts = append(parts, "dismiss_stale_reviews_on_push=skipped")
	}

	detail = strings.Join(parts, ", ")
	return okAll, detail, ""
}

func effectivePRReviewSettingsOK(dc data.DataContext, cfg *DefaultBranchPRReviewSettingsRule) (ok bool, detail string, errMsg string) {
	val, exists := dc.Get(data.DepRepoDefaultBranchEffectiveRules)
	if !exists {
		return false, "unknown", "Dependency missing"
	}
	if val == nil {
		return false, "no effective rules (nil)", ""
	}

	branchRules, okType := val.(*github.BranchRules)
	if !okType {
		return false, "unknown", "Invalid dependency type"
	}

	// In v81, PullRequest is a slice of *PullRequestBranchRule
	if len(branchRules.PullRequest) == 0 {
		return false, "no pull_request rule found", ""
	}

	// Check first pull_request rule (typically there's only one)
	for _, prRule := range branchRules.PullRequest {
		if prRule == nil {
			continue
		}

		params := prRule.Parameters

		parts := make([]string, 0, 4)
		okAll := true

		if cfg.enforceMinApprovingReviews {
			okMin := params.RequiredApprovingReviewCount >= cfg.minApprovingReviews
			okAll = okAll && okMin
			parts = append(parts, fmt.Sprintf("min_approving_reviews=%d (need >=%d)", params.RequiredApprovingReviewCount, cfg.minApprovingReviews))
		} else {
			parts = append(parts, "min_approving_reviews=skipped")
		}

		if cfg.enforceLastPushApproval {
			okLast := params.RequireLastPushApproval
			okAll = okAll && okLast
			parts = append(parts, fmt.Sprintf("require_last_push_approval=%t (need true)", params.RequireLastPushApproval))
		} else {
			parts = append(parts, "require_last_push_approval=skipped")
		}

		if cfg.enforceCodeOwnerReview {
			okOwner := params.RequireCodeOwnerReview
			okAll = okAll && okOwner
			parts = append(parts, fmt.Sprintf("require_code_owner_review=%t (need true)", params.RequireCodeOwnerReview))
		} else {
			parts = append(parts, "require_code_owner_review=skipped")
		}

		if cfg.enforceDismissStaleReviewsOnPush {
			okDismiss := params.DismissStaleReviewsOnPush
			okAll = okAll && okDismiss
			parts = append(parts, fmt.Sprintf("dismiss_stale_reviews_on_push=%t (need true)", params.DismissStaleReviewsOnPush))
		} else {
			parts = append(parts, "dismiss_stale_reviews_on_push=skipped")
		}

		detail = strings.Join(parts, ", ")
		if okAll {
			return true, detail, ""
		}
	}

	if detail == "" {
		detail = "pull_request rule present but settings not satisfied"
	}
	return false, detail, ""
}

func init() {
	r := &DefaultBranchPRReviewSettingsRule{}
	_ = r.Configure(map[string]string{})
	rules.Register(r)
}
