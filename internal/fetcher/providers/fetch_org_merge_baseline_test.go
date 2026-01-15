package providers

import (
	"testing"

	"repomedic/internal/data/models"

	"github.com/google/go-github/v81/github"
)

func TestDetermineTargetRef(t *testing.T) {
	tests := []struct {
		name  string
		repos []*github.Repository
		want  string
	}{
		{
			name:  "empty repos",
			repos: []*github.Repository{},
			want:  "refs/heads/main",
		},
		{
			name: "single repo with main",
			repos: []*github.Repository{
				{DefaultBranch: github.Ptr("main")},
			},
			want: "refs/heads/main",
		},
		{
			name: "single repo with master",
			repos: []*github.Repository{
				{DefaultBranch: github.Ptr("master")},
			},
			want: "refs/heads/master",
		},
		{
			name: "main is most common",
			repos: []*github.Repository{
				{DefaultBranch: github.Ptr("main")},
				{DefaultBranch: github.Ptr("main")},
				{DefaultBranch: github.Ptr("master")},
			},
			want: "refs/heads/main",
		},
		{
			name: "tie breaks lexicographically",
			repos: []*github.Repository{
				{DefaultBranch: github.Ptr("main")},
				{DefaultBranch: github.Ptr("develop")},
			},
			want: "refs/heads/develop", // 'd' < 'm'
		},
		{
			name: "nil repos are skipped",
			repos: []*github.Repository{
				nil,
				{DefaultBranch: github.Ptr("main")},
				nil,
			},
			want: "refs/heads/main",
		},
		{
			name: "repos with empty default branch are skipped",
			repos: []*github.Repository{
				{DefaultBranch: github.Ptr("")},
				{DefaultBranch: github.Ptr("main")},
			},
			want: "refs/heads/main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineTargetRef(tt.repos)
			if got != tt.want {
				t.Errorf("determineTargetRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRefMatchesPattern(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		pattern string
		want    bool
	}{
		// Special patterns
		{"DEFAULT_BRANCH matches branch ref", "refs/heads/main", "~DEFAULT_BRANCH", true},
		{"DEFAULT_BRANCH matches any branch", "refs/heads/develop", "~DEFAULT_BRANCH", true},
		{"DEFAULT_BRANCH does not match tag", "refs/tags/v1.0", "~DEFAULT_BRANCH", false},
		{"ALL matches everything", "refs/heads/main", "~ALL", true},
		{"ALL matches tags", "refs/tags/v1.0", "~ALL", true},

		// Wildcard patterns
		{"wildcard matches prefix", "refs/heads/main", "refs/heads/*", true},
		{"wildcard matches other branch", "refs/heads/feature/test", "refs/heads/*", true},
		{"wildcard does not match different prefix", "refs/tags/v1.0", "refs/heads/*", false},
		{"double wildcard matches nested", "refs/heads/feature/test/deep", "refs/heads/**", true},

		// Exact match
		{"exact match", "refs/heads/main", "refs/heads/main", true},
		{"exact mismatch", "refs/heads/main", "refs/heads/master", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := refMatchesPattern(tt.ref, tt.pattern)
			if got != tt.want {
				t.Errorf("refMatchesPattern(%q, %q) = %v, want %v", tt.ref, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestFilterApplicableOrgRulesets(t *testing.T) {
	targetRef := "refs/heads/main"
	branchTarget := github.RulesetTargetBranch
	tagTarget := github.RulesetTargetTag

	activeRuleset := &github.RepositoryRuleset{
		Name:        "active-ruleset",
		Enforcement: github.RulesetEnforcementActive,
		Target:      &branchTarget,
	}

	disabledRuleset := &github.RepositoryRuleset{
		Name:        "disabled-ruleset",
		Enforcement: github.RulesetEnforcementDisabled,
		Target:      &branchTarget,
	}

	evaluateRuleset := &github.RepositoryRuleset{
		Name:        "evaluate-ruleset",
		Enforcement: github.RulesetEnforcementEvaluate,
		Target:      &branchTarget,
	}

	tagRuleset := &github.RepositoryRuleset{
		Name:        "tag-ruleset",
		Enforcement: github.RulesetEnforcementActive,
		Target:      &tagTarget,
	}

	tests := []struct {
		name     string
		rulesets []*github.RepositoryRuleset
		wantLen  int
	}{
		{
			name:     "empty rulesets",
			rulesets: []*github.RepositoryRuleset{},
			wantLen:  0,
		},
		{
			name:     "only active branch rulesets pass",
			rulesets: []*github.RepositoryRuleset{activeRuleset, disabledRuleset, evaluateRuleset, tagRuleset},
			wantLen:  1,
		},
		{
			name:     "nil rulesets are skipped",
			rulesets: []*github.RepositoryRuleset{nil, activeRuleset, nil},
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterApplicableOrgRulesets(tt.rulesets, targetRef)
			if len(got) != tt.wantLen {
				t.Errorf("filterApplicableOrgRulesets() returned %d rulesets, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestRulesetMatchesRef(t *testing.T) {
	tests := []struct {
		name      string
		ruleset   *github.RepositoryRuleset
		targetRef string
		want      bool
	}{
		{
			name:      "no conditions matches all",
			ruleset:   &github.RepositoryRuleset{},
			targetRef: "refs/heads/main",
			want:      true,
		},
		{
			name: "nil ref conditions matches all",
			ruleset: &github.RepositoryRuleset{
				Conditions: &github.RepositoryRulesetConditions{},
			},
			targetRef: "refs/heads/main",
			want:      true,
		},
		{
			name: "inclusion matches",
			ruleset: &github.RepositoryRuleset{
				Conditions: &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"refs/heads/main"},
					},
				},
			},
			targetRef: "refs/heads/main",
			want:      true,
		},
		{
			name: "inclusion does not match",
			ruleset: &github.RepositoryRuleset{
				Conditions: &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"refs/heads/develop"},
					},
				},
			},
			targetRef: "refs/heads/main",
			want:      false,
		},
		{
			name: "exclusion blocks match",
			ruleset: &github.RepositoryRuleset{
				Conditions: &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"refs/heads/*"},
						Exclude: []string{"refs/heads/main"},
					},
				},
			},
			targetRef: "refs/heads/main",
			want:      false,
		},
		{
			name: "wildcard inclusion",
			ruleset: &github.RepositoryRuleset{
				Conditions: &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"refs/heads/*"},
					},
				},
			},
			targetRef: "refs/heads/main",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rulesetMatchesRef(tt.ruleset, tt.targetRef)
			if got != tt.want {
				t.Errorf("rulesetMatchesRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeriveBaselineFromRulesets_NoConstraints(t *testing.T) {
	// Ruleset with no merge-related rules.
	rulesets := []*github.RepositoryRuleset{
		{
			Name:        "no-merge-rules",
			Enforcement: github.RulesetEnforcementActive,
			Rules:       &github.RepositoryRulesetRules{},
		},
	}

	baseline, err := deriveBaselineFromRulesets(rulesets, "refs/heads/main")
	if err != nil {
		t.Fatalf("deriveBaselineFromRulesets() error = %v", err)
	}

	if baseline.State != models.BaselineStateNone {
		t.Errorf("State = %q, want %q", baseline.State, models.BaselineStateNone)
	}
}

func TestDeriveBaselineFromRulesets_LinearHistory(t *testing.T) {
	rulesets := []*github.RepositoryRuleset{
		{
			Name:        "linear-history",
			Enforcement: github.RulesetEnforcementActive,
			Rules: &github.RepositoryRulesetRules{
				RequiredLinearHistory: &github.EmptyRuleParameters{},
			},
		},
	}

	baseline, err := deriveBaselineFromRulesets(rulesets, "refs/heads/main")
	if err != nil {
		t.Fatalf("deriveBaselineFromRulesets() error = %v", err)
	}

	if baseline.State != models.BaselineStateSet {
		t.Errorf("State = %q, want %q", baseline.State, models.BaselineStateSet)
	}

	// Linear history removes merge commits, leaving squash + rebase.
	expectedMask := models.MergeMethodSquash | models.MergeMethodRebase
	if baseline.Allowed != expectedMask {
		t.Errorf("Allowed = %q, want %q", baseline.Allowed.String(), expectedMask.String())
	}
}

func TestDeriveBaselineFromRulesets_PullRequestAllowedMergeMethods(t *testing.T) {
	tests := []struct {
		name           string
		allowedMethods []github.PullRequestMergeMethod
		wantState      models.BaselineState
		wantMask       models.MergeMethodMask
	}{
		{
			name:           "merge only",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodMerge},
			wantState:      models.BaselineStateSet,
			wantMask:       models.MergeMethodMerge,
		},
		{
			name:           "squash only",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodSquash},
			wantState:      models.BaselineStateSet,
			wantMask:       models.MergeMethodSquash,
		},
		{
			name:           "rebase only",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodRebase},
			wantState:      models.BaselineStateSet,
			wantMask:       models.MergeMethodRebase,
		},
		{
			name:           "squash and rebase",
			allowedMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodSquash, github.PullRequestMergeMethodRebase},
			wantState:      models.BaselineStateSet,
			wantMask:       models.MergeMethodSquash | models.MergeMethodRebase,
		},
		{
			name:           "empty methods (no constraint)",
			allowedMethods: []github.PullRequestMergeMethod{},
			wantState:      models.BaselineStateNone,
			wantMask:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rulesets := []*github.RepositoryRuleset{
				{
					Name:        "pr-merge-methods",
					Enforcement: github.RulesetEnforcementActive,
					Rules: &github.RepositoryRulesetRules{
						PullRequest: &github.PullRequestRuleParameters{
							AllowedMergeMethods: tt.allowedMethods,
						},
					},
				},
			}

			baseline, err := deriveBaselineFromRulesets(rulesets, "refs/heads/main")
			if err != nil {
				t.Fatalf("deriveBaselineFromRulesets() error = %v", err)
			}

			if baseline.State != tt.wantState {
				t.Errorf("State = %q, want %q", baseline.State, tt.wantState)
			}

			if tt.wantState == models.BaselineStateSet && baseline.Allowed != tt.wantMask {
				t.Errorf("Allowed = %q, want %q", baseline.Allowed.String(), tt.wantMask.String())
			}
		})
	}
}

func TestDeriveBaselineFromRulesets_PullRequestWithLinearHistory(t *testing.T) {
	// pull_request allows only merge, but linear_history removes merge.
	// This should result in a conflict (empty mask).
	rulesets := []*github.RepositoryRuleset{
		{
			Name:        "conflicting-rules",
			Enforcement: github.RulesetEnforcementActive,
			Rules: &github.RepositoryRulesetRules{
				PullRequest: &github.PullRequestRuleParameters{
					AllowedMergeMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodMerge},
				},
				RequiredLinearHistory: &github.EmptyRuleParameters{},
			},
		},
	}

	baseline, err := deriveBaselineFromRulesets(rulesets, "refs/heads/main")
	if err != nil {
		t.Fatalf("deriveBaselineFromRulesets() error = %v", err)
	}

	if baseline.State != models.BaselineStateConflict {
		t.Errorf("State = %q, want %q", baseline.State, models.BaselineStateConflict)
	}
}

func TestDeriveBaselineFromRulesets_PullRequestCompatibleWithLinearHistory(t *testing.T) {
	// pull_request allows squash+rebase, linear_history removes merge.
	// These are compatible (squash+rebase remain).
	rulesets := []*github.RepositoryRuleset{
		{
			Name:        "compatible-rules",
			Enforcement: github.RulesetEnforcementActive,
			Rules: &github.RepositoryRulesetRules{
				PullRequest: &github.PullRequestRuleParameters{
					AllowedMergeMethods: []github.PullRequestMergeMethod{github.PullRequestMergeMethodSquash, github.PullRequestMergeMethodRebase},
				},
				RequiredLinearHistory: &github.EmptyRuleParameters{},
			},
		},
	}

	baseline, err := deriveBaselineFromRulesets(rulesets, "refs/heads/main")
	if err != nil {
		t.Fatalf("deriveBaselineFromRulesets() error = %v", err)
	}

	if baseline.State != models.BaselineStateSet {
		t.Errorf("State = %q, want %q", baseline.State, models.BaselineStateSet)
	}

	expectedMask := models.MergeMethodSquash | models.MergeMethodRebase
	if baseline.Allowed != expectedMask {
		t.Errorf("Allowed = %q, want %q", baseline.Allowed.String(), expectedMask.String())
	}
}
