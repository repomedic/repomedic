package checks

import (
	"context"
	"encoding/json"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestDefaultBranchRequiredStatusChecks_Evaluate(t *testing.T) {
	rule := &DefaultBranchRequiredStatusChecks{}
	repo := &github.Repository{
		FullName:      github.String("org/repo"),
		DefaultBranch: github.String("main"),
	}

	makeRule := func(checks []string) *github.RepositoryRule {
		params := requiredStatusChecksParams{
			RequiredStatusChecks: make([]struct {
				Context string `json:"context"`
			}, len(checks)),
		}
		for i, c := range checks {
			params.RequiredStatusChecks[i].Context = c
		}
		b, _ := json.Marshal(params)
		raw := json.RawMessage(b)
		return &github.RepositoryRule{
			Type:          "required_status_checks",
			Parameters:    &raw,
			RulesetSource: "test",
		}

	}

	tests := []struct {
		name           string
		config         map[string]string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
		expectedMsg    string // partial match
	}{
		{
			name:           "missing dependency",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
			expectedMsg:    "missing dependency",
		},
		{
			name: "wrong type",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: "wrong",
			},
			expectedStatus: rules.StatusError,
			expectedMsg:    "unexpected type",
		},
		{
			name: "no rules",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
			expectedMsg:    "Default branch does not require status checks",
		},
		{
			name: "rule present, default config (allow_any=true, min_count=1), 1 check",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					makeRule([]string{"ci/test"}),
				},
			},
			expectedStatus: rules.StatusPass,
			expectedMsg:    "Default branch requires status checks",
		},
		{
			name: "rule present, min_count=2, 1 check",
			config: map[string]string{
				"min_count": "2",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					makeRule([]string{"ci/test"}),
				},
			},
			expectedStatus: rules.StatusFail,
			expectedMsg:    "only 1 are configured",
		},
		{
			name: "rule present, allow_any=false, required check present",
			config: map[string]string{
				"allow_any": "false",
				"required":  "ci/test",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					makeRule([]string{"ci/test"}),
				},
			},
			expectedStatus: rules.StatusPass,
			expectedMsg:    "Default branch requires all specified status checks",
		},
		{
			name: "rule present, allow_any=false, required check missing",
			config: map[string]string{
				"allow_any": "false",
				"required":  "ci/test",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					makeRule([]string{"other"}),
				},
			},
			expectedStatus: rules.StatusFail,
			expectedMsg:    "missing required status checks: ci/test",
		},
		{
			name: "rule present, allow_any=false, required check case insensitive",
			config: map[string]string{
				"allow_any": "false",
				"required":  "CI/TEST",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					makeRule([]string{"ci/test"}),
				},
			},
			expectedStatus: rules.StatusPass,
			expectedMsg:    "Default branch requires all specified status checks",
		},
		{
			name: "rule present, allow_any=false, multiple required, one missing",
			config: map[string]string{
				"allow_any": "false",
				"required":  "ci/test, ci/lint",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					makeRule([]string{"ci/test"}),
				},
			},
			expectedStatus: rules.StatusFail,
			expectedMsg:    "missing required status checks: ci/lint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := data.NewMapDataContext(tt.data)
			if tt.config != nil {
				if err := rule.Configure(tt.config); err != nil {
					t.Fatalf("Configure error: %v", err)
				}
			} else {
				if err := rule.Configure(map[string]string{}); err != nil {
					t.Fatalf("Configure error: %v", err)
				}
			}

			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("want status %v, got %v (msg: %s)", tt.expectedStatus, result.Status, result.Message)
			}
			if tt.expectedMsg != "" && !strings.Contains(result.Message, tt.expectedMsg) {
				t.Errorf("want message containing %q, got %q", tt.expectedMsg, result.Message)
			}
		})
	}
}
