package checks

import (
	"context"
	"repomedic/internal/rules"
	"encoding/json"
	"repomedic/internal/data"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestDefaultBranchPRReviewSettingsRule_Evaluate(t *testing.T) {
	repo := &github.Repository{FullName: github.String("acme/repo"), DefaultBranch: github.String("main")}

	okParamsBytes, _ := json.Marshal(github.PullRequestRuleParameters{
		RequiredApprovingReviewCount: 1,
		RequireLastPushApproval:      true,
		RequireCodeOwnerReview:       true,
		DismissStaleReviewsOnPush:    true,
	})
	okRaw := json.RawMessage(okParamsBytes)

	badParamsBytes, _ := json.Marshal(github.PullRequestRuleParameters{
		RequiredApprovingReviewCount: 0,
		RequireLastPushApproval:      false,
		RequireCodeOwnerReview:       false,
		DismissStaleReviewsOnPush:    false,
	})
	badRaw := json.RawMessage(badParamsBytes)

	tests := []struct {
		name           string
		configure      map[string]string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name:      "PASS classic - defaults require all four",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 1,
					RequireLastPushApproval:      true,
					RequireCodeOwnerReviews:      true,
					DismissStaleReviews:          true,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name:      "FAIL classic - approvals 0",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 0,
					RequireLastPushApproval:      true,
					RequireCodeOwnerReviews:      true,
					DismissStaleReviews:          true,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:      "FAIL classic - last push approval false",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 1,
					RequireLastPushApproval:      false,
					RequireCodeOwnerReviews:      true,
					DismissStaleReviews:          true,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:      "FAIL classic - code owner review false",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 1,
					RequireLastPushApproval:      true,
					RequireCodeOwnerReviews:      false,
					DismissStaleReviews:          true,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:      "FAIL classic - dismiss stale reviews false",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 1,
					RequireLastPushApproval:      true,
					RequireCodeOwnerReviews:      true,
					DismissStaleReviews:          false,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:      "PASS effective rules - pull_request rule satisfies",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{{
					Type:       "pull_request",
					Parameters: &okRaw,
				}},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name:      "FAIL effective rules - pull_request rule present but not satisfied",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{{
					Type:       "pull_request",
					Parameters: &badRaw,
				}},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "PASS classic - code owner check toggled off",
			configure: map[string]string{
				"enforce_code_owner_review": "false",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 1,
					RequireLastPushApproval:      true,
					RequireCodeOwnerReviews:      false,
					DismissStaleReviews:          true,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "FAIL classic - min reviewers set to 2",
			configure: map[string]string{
				"min_approving_reviews": "2",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 1,
					RequireLastPushApproval:      true,
					RequireCodeOwnerReviews:      true,
					DismissStaleReviews:          true,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "PASS classic - min reviewers set to 2 and satisfied",
			configure: map[string]string{
				"min_approving_reviews": "2",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 2,
					RequireLastPushApproval:      true,
					RequireCodeOwnerReviews:      true,
					DismissStaleReviews:          true,
				}},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name:           "PASS all checks toggled off",
			configure:      map[string]string{"enforce_min_approving_reviews": "false", "enforce_last_push_approval": "false", "enforce_code_owner_review": "false", "enforce_dismiss_stale_reviews_on_push": "false"},
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusPass,
		},
		{
			name:           "ERROR missing dependencies",
			configure:      map[string]string{},
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name:      "ERROR wrong type - classic protection",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: "nope",
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusError,
		},
		{
			name:      "ERROR wrong type - effective rules",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    "nope",
			},
			expectedStatus: rules.StatusError,
		},
		{
			name:      "ERROR pull_request rule missing parameters",
			configure: map[string]string{},
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{{
					Type:       "pull_request",
					Parameters: nil,
				}},
			},
			expectedStatus: rules.StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &DefaultBranchPRReviewSettingsRule{}
			if err := rule.Configure(tt.configure); err != nil {
				t.Fatalf("Configure error: %v", err)
			}
			dc := data.NewMapDataContext(tt.data)
			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.Status != tt.expectedStatus {
				t.Fatalf("want %v, got %v (message=%q)", tt.expectedStatus, result.Status, result.Message)
			}
			if result.RuleID != rule.ID() {
				t.Fatalf("expected rule ID %q, got %q", rule.ID(), result.RuleID)
			}
		})
	}
}
