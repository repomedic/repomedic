package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strings"
	"testing"

	"github.com/google/go-github/v81/github"
)

func TestRulesetsActiveRule_Evaluate(t *testing.T) {
	repo := &github.Repository{FullName: github.Ptr("acme/repo"), DefaultBranch: github.Ptr("main")}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
		wantMsgContain string
	}{
		{
			name: "PASS when all rulesets are active",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.RepositoryRuleset{
					{ID: github.Ptr(int64(1)), Name: "main-branch-protection", Enforcement: github.RulesetEnforcementActive},
					{ID: github.Ptr(int64(2)), Name: "release-protection", Enforcement: github.RulesetEnforcementActive},
				},
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "All 2 ruleset(s) are active",
		},
		{
			name: "PASS when no rulesets configured (empty slice)",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.RepositoryRuleset{},
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "No rulesets configured",
		},
		{
			name: "PASS when no rulesets configured (nil)",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: nil,
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "No rulesets configured",
		},
		{
			name: "FAIL when ruleset is disabled",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.RepositoryRuleset{
					{ID: github.Ptr(int64(1)), Name: "disabled-ruleset", Enforcement: github.RulesetEnforcementDisabled},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "disabled-ruleset (disabled)",
		},
		{
			name: "FAIL when ruleset is in evaluate mode",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.RepositoryRuleset{
					{ID: github.Ptr(int64(1)), Name: "test-ruleset", Enforcement: github.RulesetEnforcementEvaluate},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "test-ruleset (evaluate)",
		},
		{
			name: "FAIL when multiple rulesets are not active",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.RepositoryRuleset{
					{ID: github.Ptr(int64(1)), Name: "active-ruleset", Enforcement: github.RulesetEnforcementActive},
					{ID: github.Ptr(int64(2)), Name: "disabled-ruleset", Enforcement: github.RulesetEnforcementDisabled},
					{ID: github.Ptr(int64(3)), Name: "evaluate-ruleset", Enforcement: github.RulesetEnforcementEvaluate},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "Found 2 non-active ruleset(s)",
		},
		{
			name: "FAIL shows ID when name is empty",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.RepositoryRuleset{
					{ID: github.Ptr(int64(123)), Name: "", Enforcement: github.RulesetEnforcementDisabled},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "id=123 (disabled)",
		},
		{
			name:           "ERROR when dependency is missing",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
			wantMsgContain: "Dependency missing",
		},
		{
			name: "ERROR when dependency is wrong type",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: "invalid",
			},
			expectedStatus: rules.StatusError,
			wantMsgContain: "Invalid dependency type",
		},
		{
			name: "PASS skips nil rulesets in slice",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.RepositoryRuleset{
					nil,
					{ID: github.Ptr(int64(1)), Name: "active-ruleset", Enforcement: github.RulesetEnforcementActive},
					nil,
				},
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "All 1 ruleset(s) are active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &RulesetsActiveRule{}

			dc := data.NewMapDataContext(tt.data)
			res, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if res.Status != tt.expectedStatus {
				t.Fatalf("want status %v, got %v (message: %s)", tt.expectedStatus, res.Status, res.Message)
			}
			if tt.wantMsgContain != "" && !strings.Contains(res.Message, tt.wantMsgContain) {
				t.Fatalf("want message containing %q, got %q", tt.wantMsgContain, res.Message)
			}
		})
	}
}

func TestRulesetsActiveRule_ID(t *testing.T) {
	rule := &RulesetsActiveRule{}
	if rule.ID() != "rulesets-active" {
		t.Fatalf("want ID 'rulesets-active', got %q", rule.ID())
	}
}

func TestRulesetsActiveRule_Dependencies(t *testing.T) {
	rule := &RulesetsActiveRule{}
	repo := &github.Repository{FullName: github.Ptr("acme/repo")}

	deps, err := rule.Dependencies(context.Background(), repo)
	if err != nil {
		t.Fatalf("Dependencies error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("want 1 dependency, got %d", len(deps))
	}
	if deps[0] != data.DepRepoAllRulesets {
		t.Fatalf("want dependency %q, got %q", data.DepRepoAllRulesets, deps[0])
	}
}
