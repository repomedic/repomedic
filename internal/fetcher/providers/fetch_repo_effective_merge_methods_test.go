package providers

import (
	"testing"

	"repomedic/internal/data/models"

	"github.com/google/go-github/v81/github"
)

func TestApplyRulesetConstraints(t *testing.T) {
	targetRef := "refs/heads/main"
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase
	branchTarget := github.RulesetTargetBranch
	tagTarget := github.RulesetTargetTag

	tests := []struct {
		name     string
		rulesets []*github.RepositoryRuleset
		want     models.MergeMethodMask
	}{
		{
			name:     "empty rulesets",
			rulesets: []*github.RepositoryRuleset{},
			want:     allMethods,
		},
		{
			name: "inactive ruleset is ignored",
			rulesets: []*github.RepositoryRuleset{
				{
					Enforcement: github.RulesetEnforcementDisabled,
					Target:      &branchTarget,
					Rules:       &github.RepositoryRulesetRules{RequiredLinearHistory: &github.EmptyRuleParameters{}},
				},
			},
			want: allMethods,
		},
		{
			name: "linear history removes merge",
			rulesets: []*github.RepositoryRuleset{
				{
					Enforcement: github.RulesetEnforcementActive,
					Target:      &branchTarget,
					Rules:       &github.RepositoryRulesetRules{RequiredLinearHistory: &github.EmptyRuleParameters{}},
				},
			},
			want: models.MergeMethodSquash | models.MergeMethodRebase,
		},
		{
			name: "tag ruleset is ignored",
			rulesets: []*github.RepositoryRuleset{
				{
					Enforcement: github.RulesetEnforcementActive,
					Target:      &tagTarget,
					Rules:       &github.RepositoryRulesetRules{RequiredLinearHistory: &github.EmptyRuleParameters{}},
				},
			},
			want: allMethods,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRulesetConstraints(tt.rulesets, targetRef, allMethods)
			if got != tt.want {
				t.Errorf("applyRulesetConstraints() = %v (%s), want %v (%s)",
					got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestApplyRulesConstraints(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	tests := []struct {
		name  string
		rules *github.RepositoryRulesetRules
		want  models.MergeMethodMask
	}{
		{
			name:  "empty rules",
			rules: &github.RepositoryRulesetRules{},
			want:  allMethods,
		},
		{
			name:  "nil rules",
			rules: nil,
			want:  allMethods,
		},
		{
			name: "required_linear_history removes merge",
			rules: &github.RepositoryRulesetRules{
				RequiredLinearHistory: &github.EmptyRuleParameters{},
			},
			want: models.MergeMethodSquash | models.MergeMethodRebase,
		},
		{
			name: "merge_queue with squash",
			rules: &github.RepositoryRulesetRules{
				MergeQueue: &github.MergeQueueRuleParameters{
					MergeMethod: github.MergeQueueMergeMethodSquash,
				},
			},
			want: models.MergeMethodSquash,
		},
		{
			name: "combined linear history and merge queue",
			rules: &github.RepositoryRulesetRules{
				RequiredLinearHistory: &github.EmptyRuleParameters{},
				MergeQueue: &github.MergeQueueRuleParameters{
					MergeMethod: github.MergeQueueMergeMethodSquash,
				},
			},
			want: models.MergeMethodSquash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRulesConstraints(tt.rules, allMethods)
			if got != tt.want {
				t.Errorf("applyRulesConstraints() = %v (%s), want %v (%s)",
					got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestApplyRulesConstraints_MergeQueueMethods(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	tests := []struct {
		name   string
		method github.MergeQueueMergeMethod
		want   models.MergeMethodMask
	}{
		{"MERGE", github.MergeQueueMergeMethodMerge, models.MergeMethodMerge},
		{"SQUASH", github.MergeQueueMergeMethodSquash, models.MergeMethodSquash},
		{"REBASE", github.MergeQueueMergeMethodRebase, models.MergeMethodRebase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := &github.RepositoryRulesetRules{
				MergeQueue: &github.MergeQueueRuleParameters{
					MergeMethod: tt.method,
				},
			}

			got := applyRulesConstraints(rules, allMethods)
			if got != tt.want {
				t.Errorf("applyRulesConstraints() = %v (%s), want %v (%s)",
					got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestApplyRulesConstraints_ConflictingRules(t *testing.T) {
	// Merge queue requires MERGE, but linear history prohibits it.
	rules := &github.RepositoryRulesetRules{
		MergeQueue: &github.MergeQueueRuleParameters{
			MergeMethod: github.MergeQueueMergeMethodMerge,
		},
		RequiredLinearHistory: &github.EmptyRuleParameters{},
	}

	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase
	got := applyRulesConstraints(rules, allMethods)

	// Result should be 0 (no valid methods).
	if got != 0 {
		t.Errorf("applyRulesConstraints() = %v (%s), want 0 (empty)",
			got, got.String())
	}
}

func TestApplyRulesConstraints_PullRequestAllowedMergeMethods(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	tests := []struct {
		name           string
		allowedMethods []github.PullRequestMergeMethod
		want           models.MergeMethodMask
	}{
		{
			name:           "merge only",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodMerge},
			want:           models.MergeMethodMerge,
		},
		{
			name:           "squash only",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodSquash},
			want:           models.MergeMethodSquash,
		},
		{
			name:           "rebase only",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodRebase},
			want:           models.MergeMethodRebase,
		},
		{
			name:           "squash and rebase",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodSquash, github.PullRequestMergeMethodRebase},
			want:           models.MergeMethodSquash | models.MergeMethodRebase,
		},
		{
			name:           "all methods",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodMerge, github.PullRequestMergeMethodSquash, github.PullRequestMergeMethodRebase},
			want:           allMethods,
		},
		{
			name:           "empty methods preserves all",
			allowedMethods: []github.PullRequestMergeMethod{},
			want:           allMethods,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := &github.RepositoryRulesetRules{
				PullRequest: &github.PullRequestRuleParameters{
					AllowedMergeMethods: tt.allowedMethods,
				},
			}

			got := applyRulesConstraints(rules, allMethods)
			if got != tt.want {
				t.Errorf("applyRulesConstraints() = %v (%s), want %v (%s)",
					got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestApplyRulesConstraints_PullRequestWithOtherRules(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	// pull_request allows squash+rebase, linear_history removes merge.
	// Combined effect should be squash+rebase.
	rules := &github.RepositoryRulesetRules{
		PullRequest: &github.PullRequestRuleParameters{
			AllowedMergeMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodSquash, github.PullRequestMergeMethodRebase},
		},
		RequiredLinearHistory: &github.EmptyRuleParameters{},
	}

	got := applyRulesConstraints(rules, allMethods)
	want := models.MergeMethodSquash | models.MergeMethodRebase
	if got != want {
		t.Errorf("applyRulesConstraints() = %v (%s), want %v (%s)",
			got, got.String(), want, want.String())
	}
}

func TestApplyRulesConstraints_PullRequestConflict(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	// pull_request only allows merge, but linear_history removes merge.
	// Result should be 0 (conflict).
	rules := &github.RepositoryRulesetRules{
		PullRequest: &github.PullRequestRuleParameters{
			AllowedMergeMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodMerge},
		},
		RequiredLinearHistory: &github.EmptyRuleParameters{},
	}

	got := applyRulesConstraints(rules, allMethods)
	if got != 0 {
		t.Errorf("applyRulesConstraints() = %v (%s), want 0 (empty)",
			got, got.String())
	}
}
