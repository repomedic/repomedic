package checks

import (
	"context"
	"repomedic/internal/rules"
	"repomedic/internal/data"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestRulesetsActiveRule_Evaluate(t *testing.T) {
	repo := &github.Repository{FullName: github.String("acme/repo"), DefaultBranch: github.String("main")}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
		wantMsgContain string
	}{
		{
			name: "PASS when all rulesets are active",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.Ruleset{
					{ID: github.Int64(1), Name: "main-branch-protection", Enforcement: "active"},
					{ID: github.Int64(2), Name: "release-protection", Enforcement: "active"},
				},
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "All 2 ruleset(s) are active",
		},
		{
			name: "PASS when no rulesets configured (empty slice)",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.Ruleset{},
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
				data.DepRepoAllRulesets: []*github.Ruleset{
					{ID: github.Int64(1), Name: "disabled-ruleset", Enforcement: "disabled"},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "disabled-ruleset (disabled)",
		},
		{
			name: "FAIL when ruleset is in evaluate mode",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.Ruleset{
					{ID: github.Int64(1), Name: "test-ruleset", Enforcement: "evaluate"},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "test-ruleset (evaluate)",
		},
		{
			name: "FAIL when multiple rulesets are not active",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.Ruleset{
					{ID: github.Int64(1), Name: "active-ruleset", Enforcement: "active"},
					{ID: github.Int64(2), Name: "disabled-ruleset", Enforcement: "disabled"},
					{ID: github.Int64(3), Name: "evaluate-ruleset", Enforcement: "evaluate"},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "Found 2 non-active ruleset(s)",
		},
		{
			name: "FAIL when ruleset has uppercase enforcement value",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.Ruleset{
					{ID: github.Int64(1), Name: "test", Enforcement: "EVALUATE"},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "test (evaluate)",
		},
		{
			name: "PASS when ruleset has uppercase ACTIVE enforcement",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.Ruleset{
					{ID: github.Int64(1), Name: "test", Enforcement: "ACTIVE"},
				},
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "All 1 ruleset(s) are active",
		},
		{
			name: "FAIL shows ID when name is empty",
			data: map[data.DependencyKey]any{
				data.DepRepoAllRulesets: []*github.Ruleset{
					{ID: github.Int64(123), Name: "", Enforcement: "disabled"},
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
				data.DepRepoAllRulesets: []*github.Ruleset{
					nil,
					{ID: github.Int64(1), Name: "active-ruleset", Enforcement: "active"},
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
			if tt.wantMsgContain != "" && !contains(res.Message, tt.wantMsgContain) {
				t.Fatalf("want message containing %q, got %q", tt.wantMsgContain, res.Message)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRulesetsActiveRule_ID(t *testing.T) {
	rule := &RulesetsActiveRule{}
	if rule.ID() != "rulesets-active" {
		t.Fatalf("want ID 'rulesets-active', got %q", rule.ID())
	}
}

func TestRulesetsActiveRule_Dependencies(t *testing.T) {
	rule := &RulesetsActiveRule{}
	repo := &github.Repository{FullName: github.String("acme/repo")}

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
