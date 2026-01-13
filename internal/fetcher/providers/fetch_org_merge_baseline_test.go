package providers

import (
	"testing"

	"repomedic/internal/data/models"

	"github.com/google/go-github/v66/github"
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
				{DefaultBranch: github.String("main")},
			},
			want: "refs/heads/main",
		},
		{
			name: "single repo with master",
			repos: []*github.Repository{
				{DefaultBranch: github.String("master")},
			},
			want: "refs/heads/master",
		},
		{
			name: "main is most common",
			repos: []*github.Repository{
				{DefaultBranch: github.String("main")},
				{DefaultBranch: github.String("main")},
				{DefaultBranch: github.String("master")},
			},
			want: "refs/heads/main",
		},
		{
			name: "tie breaks lexicographically",
			repos: []*github.Repository{
				{DefaultBranch: github.String("main")},
				{DefaultBranch: github.String("develop")},
			},
			want: "refs/heads/develop", // 'd' < 'm'
		},
		{
			name: "nil repos are skipped",
			repos: []*github.Repository{
				nil,
				{DefaultBranch: github.String("main")},
				nil,
			},
			want: "refs/heads/main",
		},
		{
			name: "repos with empty default branch are skipped",
			repos: []*github.Repository{
				{DefaultBranch: github.String("")},
				{DefaultBranch: github.String("main")},
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

	activeRuleset := &github.Ruleset{
		Name:        "active-ruleset",
		Enforcement: "active",
		Target:      github.String("branch"),
	}

	disabledRuleset := &github.Ruleset{
		Name:        "disabled-ruleset",
		Enforcement: "disabled",
		Target:      github.String("branch"),
	}

	evaluateRuleset := &github.Ruleset{
		Name:        "evaluate-ruleset",
		Enforcement: "evaluate",
		Target:      github.String("branch"),
	}

	tagRuleset := &github.Ruleset{
		Name:        "tag-ruleset",
		Enforcement: "active",
		Target:      github.String("tag"),
	}

	tests := []struct {
		name     string
		rulesets []*github.Ruleset
		wantLen  int
	}{
		{
			name:     "empty rulesets",
			rulesets: []*github.Ruleset{},
			wantLen:  0,
		},
		{
			name:     "only active branch rulesets pass",
			rulesets: []*github.Ruleset{activeRuleset, disabledRuleset, evaluateRuleset, tagRuleset},
			wantLen:  1,
		},
		{
			name:     "nil rulesets are skipped",
			rulesets: []*github.Ruleset{nil, activeRuleset, nil},
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
		ruleset   *github.Ruleset
		targetRef string
		want      bool
	}{
		{
			name:      "no conditions matches all",
			ruleset:   &github.Ruleset{},
			targetRef: "refs/heads/main",
			want:      true,
		},
		{
			name: "nil ref conditions matches all",
			ruleset: &github.Ruleset{
				Conditions: &github.RulesetConditions{},
			},
			targetRef: "refs/heads/main",
			want:      true,
		},
		{
			name: "inclusion matches",
			ruleset: &github.Ruleset{
				Conditions: &github.RulesetConditions{
					RefName: &github.RulesetRefConditionParameters{
						Include: []string{"refs/heads/main"},
					},
				},
			},
			targetRef: "refs/heads/main",
			want:      true,
		},
		{
			name: "inclusion does not match",
			ruleset: &github.Ruleset{
				Conditions: &github.RulesetConditions{
					RefName: &github.RulesetRefConditionParameters{
						Include: []string{"refs/heads/develop"},
					},
				},
			},
			targetRef: "refs/heads/main",
			want:      false,
		},
		{
			name: "exclusion blocks match",
			ruleset: &github.Ruleset{
				Conditions: &github.RulesetConditions{
					RefName: &github.RulesetRefConditionParameters{
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
			ruleset: &github.Ruleset{
				Conditions: &github.RulesetConditions{
					RefName: &github.RulesetRefConditionParameters{
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
	rulesets := []*github.Ruleset{
		{
			Name:        "no-merge-rules",
			Enforcement: "active",
			Rules:       []*github.RepositoryRule{},
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
	rulesets := []*github.Ruleset{
		{
			Name:        "linear-history",
			Enforcement: "active",
			Rules: []*github.RepositoryRule{
				{Type: "required_linear_history"},
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
