package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestConsistentDefaultBranchMergeMethodsRule_Evaluate(t *testing.T) {
	repo := &github.Repository{FullName: github.String("acme/repo"), DefaultBranch: github.String("main")}

	tests := []struct {
		name           string
		configure      map[string]string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
		wantMsgContain string
	}{
		// PASS cases
		{
			name:      "PASS when effective matches baseline (squash only)",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
				data.DepMergeBaseline: &models.MergeBaseline{
					State:   models.BaselineStateSet,
					Source:  models.BaselineSourceOrganizationRuleset,
					Allowed: models.MergeMethodSquash,
				},
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "Merge methods match baseline",
		},
		{
			name:      "PASS when effective matches baseline (multiple methods)",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash | models.MergeMethodRebase,
				data.DepMergeBaseline: &models.MergeBaseline{
					State:   models.BaselineStateSet,
					Source:  models.BaselineSourceConvention,
					Allowed: models.MergeMethodSquash | models.MergeMethodRebase,
				},
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "Merge methods match baseline",
		},
		{
			name: "PASS when effective matches required_configuration",
			configure: map[string]string{
				"required_configuration": "squash,rebase",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash | models.MergeMethodRebase,
				// No baseline needed when required_configuration is set
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "Merge methods match baseline",
		},
		{
			name: "PASS when effective matches required_configuration (single method)",
			configure: map[string]string{
				"required_configuration": "squash",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
			},
			expectedStatus: rules.StatusPass,
			wantMsgContain: "Merge methods match baseline",
		},

		// FAIL cases
		{
			name:      "FAIL when effective differs from baseline",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase,
				data.DepMergeBaseline: &models.MergeBaseline{
					State:   models.BaselineStateSet,
					Source:  models.BaselineSourceOrganizationRuleset,
					Allowed: models.MergeMethodSquash,
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "Merge methods mismatch",
		},
		{
			name:      "FAIL when baseline is in conflict state",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
				data.DepMergeBaseline: &models.MergeBaseline{
					State:    models.BaselineStateConflict,
					Evidence: []string{"multiple incompatible rulesets"},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "Merge baseline conflict detected",
		},
		{
			name:      "FAIL when baseline conflict includes evidence",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
				data.DepMergeBaseline: &models.MergeBaseline{
					State:    models.BaselineStateConflict,
					Evidence: []string{"ruleset A requires squash", "ruleset B requires rebase"},
				},
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "ruleset A requires squash",
		},
		{
			name: "FAIL when effective differs from required_configuration",
			configure: map[string]string{
				"required_configuration": "squash",
			},
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodMerge | models.MergeMethodSquash,
			},
			expectedStatus: rules.StatusFail,
			wantMsgContain: "Merge methods mismatch",
		},

		// SKIP cases
		{
			name:      "SKIP when baseline is nil",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
				data.DepMergeBaseline:             nil,
			},
			expectedStatus: rules.StatusSkipped,
			wantMsgContain: "No merge baseline available",
		},
		{
			name:      "SKIP when baseline state is none",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
				data.DepMergeBaseline: &models.MergeBaseline{
					State: models.BaselineStateNone,
				},
			},
			expectedStatus: rules.StatusSkipped,
			wantMsgContain: "No merge baseline could be determined",
		},

		// ERROR cases
		{
			name:      "ERROR when effective merge methods dependency missing",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepMergeBaseline: &models.MergeBaseline{
					State:   models.BaselineStateSet,
					Source:  models.BaselineSourceOrganizationRuleset,
					Allowed: models.MergeMethodSquash,
				},
			},
			expectedStatus: rules.StatusError,
			wantMsgContain: "Dependency missing: repo effective merge methods",
		},
		{
			name:      "ERROR when merge baseline dependency missing",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
			},
			expectedStatus: rules.StatusError,
			wantMsgContain: "Dependency missing: merge baseline",
		},
		{
			name:      "ERROR when effective merge methods has wrong type",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: "not a mask",
				data.DepMergeBaseline: &models.MergeBaseline{
					State:   models.BaselineStateSet,
					Source:  models.BaselineSourceOrganizationRuleset,
					Allowed: models.MergeMethodSquash,
				},
			},
			expectedStatus: rules.StatusError,
			wantMsgContain: "Invalid dependency type: repo effective merge methods",
		},
		{
			name:      "ERROR when merge baseline has wrong type",
			configure: nil,
			data: map[data.DependencyKey]any{
				data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
				data.DepMergeBaseline:             "not a baseline",
			},
			expectedStatus: rules.StatusError,
			wantMsgContain: "Invalid dependency type: merge baseline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &ConsistentDefaultBranchMergeMethodsRule{}

			// Configure the rule if options provided
			if tt.configure != nil {
				if err := rule.Configure(tt.configure); err != nil {
					t.Fatalf("Configure error: %v", err)
				}
			}

			dc := data.NewMapDataContext(tt.data)
			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.expectedStatus)
			}

			if tt.wantMsgContain != "" && !strings.Contains(result.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want containing %q", result.Message, tt.wantMsgContain)
			}
		})
	}
}

func TestConsistentDefaultBranchMergeMethodsRule_Configure(t *testing.T) {
	tests := []struct {
		name        string
		opts        map[string]string
		wantErr     bool
		wantErrText string
		wantMask    models.MergeMethodMask
		wantHasConf bool
	}{
		{
			name:        "no config uses baseline",
			opts:        nil,
			wantErr:     false,
			wantMask:    0,
			wantHasConf: false,
		},
		{
			name:        "empty config uses baseline",
			opts:        map[string]string{},
			wantErr:     false,
			wantMask:    0,
			wantHasConf: false,
		},
		{
			name: "empty required_configuration uses baseline",
			opts: map[string]string{
				"required_configuration": "",
			},
			wantErr:     false,
			wantMask:    0,
			wantHasConf: false,
		},
		{
			name: "whitespace-only required_configuration uses baseline",
			opts: map[string]string{
				"required_configuration": "   ",
			},
			wantErr:     false,
			wantMask:    0,
			wantHasConf: false,
		},
		{
			name: "single method: squash",
			opts: map[string]string{
				"required_configuration": "squash",
			},
			wantErr:     false,
			wantMask:    models.MergeMethodSquash,
			wantHasConf: true,
		},
		{
			name: "single method: merge",
			opts: map[string]string{
				"required_configuration": "merge",
			},
			wantErr:     false,
			wantMask:    models.MergeMethodMerge,
			wantHasConf: true,
		},
		{
			name: "single method: rebase",
			opts: map[string]string{
				"required_configuration": "rebase",
			},
			wantErr:     false,
			wantMask:    models.MergeMethodRebase,
			wantHasConf: true,
		},
		{
			name: "multiple methods: squash,rebase",
			opts: map[string]string{
				"required_configuration": "squash,rebase",
			},
			wantErr:     false,
			wantMask:    models.MergeMethodSquash | models.MergeMethodRebase,
			wantHasConf: true,
		},
		{
			name: "all methods",
			opts: map[string]string{
				"required_configuration": "merge,squash,rebase",
			},
			wantErr:     false,
			wantMask:    models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase,
			wantHasConf: true,
		},
		{
			name: "handles whitespace in values",
			opts: map[string]string{
				"required_configuration": " squash , rebase ",
			},
			wantErr:     false,
			wantMask:    models.MergeMethodSquash | models.MergeMethodRebase,
			wantHasConf: true,
		},
		{
			name: "handles case insensitivity",
			opts: map[string]string{
				"required_configuration": "SQUASH,Rebase",
			},
			wantErr:     false,
			wantMask:    models.MergeMethodSquash | models.MergeMethodRebase,
			wantHasConf: true,
		},
		{
			name: "invalid method returns error",
			opts: map[string]string{
				"required_configuration": "squash,invalid",
			},
			wantErr:     true,
			wantErrText: "unknown merge method",
		},
		{
			name: "only commas not allowed",
			opts: map[string]string{
				"required_configuration": ",,,",
			},
			wantErr:     true,
			wantErrText: "must specify at least one merge method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &ConsistentDefaultBranchMergeMethodsRule{}
			err := rule.Configure(tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErrText)
				}
				return
			}

			if err != nil {
				t.Fatalf("Configure error: %v", err)
			}

			if rule.requiredConfiguration != tt.wantMask {
				t.Errorf("requiredConfiguration = %v, want %v", rule.requiredConfiguration, tt.wantMask)
			}

			if rule.hasRequiredConfig != tt.wantHasConf {
				t.Errorf("hasRequiredConfig = %v, want %v", rule.hasRequiredConfig, tt.wantHasConf)
			}
		})
	}
}

func TestConsistentDefaultBranchMergeMethodsRule_Dependencies(t *testing.T) {
	repo := &github.Repository{FullName: github.String("acme/repo")}

	t.Run("without required_configuration includes baseline", func(t *testing.T) {
		rule := &ConsistentDefaultBranchMergeMethodsRule{}
		deps, err := rule.Dependencies(context.Background(), repo)
		if err != nil {
			t.Fatalf("Dependencies error: %v", err)
		}

		hasMergeBaseline := false
		hasEffective := false
		for _, d := range deps {
			if d == data.DepMergeBaseline {
				hasMergeBaseline = true
			}
			if d == data.DepRepoEffectiveMergeMethods {
				hasEffective = true
			}
		}

		if !hasMergeBaseline {
			t.Error("expected DepMergeBaseline in dependencies")
		}
		if !hasEffective {
			t.Error("expected DepRepoEffectiveMergeMethods in dependencies")
		}
	})

	t.Run("with required_configuration excludes baseline", func(t *testing.T) {
		rule := &ConsistentDefaultBranchMergeMethodsRule{}
		err := rule.Configure(map[string]string{
			"required_configuration": "squash",
		})
		if err != nil {
			t.Fatalf("Configure error: %v", err)
		}

		deps, err := rule.Dependencies(context.Background(), repo)
		if err != nil {
			t.Fatalf("Dependencies error: %v", err)
		}

		hasMergeBaseline := false
		hasEffective := false
		for _, d := range deps {
			if d == data.DepMergeBaseline {
				hasMergeBaseline = true
			}
			if d == data.DepRepoEffectiveMergeMethods {
				hasEffective = true
			}
		}

		if hasMergeBaseline {
			t.Error("expected DepMergeBaseline to NOT be in dependencies when required_configuration is set")
		}
		if !hasEffective {
			t.Error("expected DepRepoEffectiveMergeMethods in dependencies")
		}
	})
}

func TestConsistentDefaultBranchMergeMethodsRule_Metadata(t *testing.T) {
	repo := &github.Repository{FullName: github.String("acme/repo"), DefaultBranch: github.String("main")}
	rule := &ConsistentDefaultBranchMergeMethodsRule{}

	t.Run("PASS result includes metadata", func(t *testing.T) {
		dc := data.NewMapDataContext(map[data.DependencyKey]any{
			data.DepRepoEffectiveMergeMethods: models.MergeMethodSquash,
			data.DepMergeBaseline: &models.MergeBaseline{
				State:   models.BaselineStateSet,
				Source:  models.BaselineSourceOrganizationRuleset,
				Allowed: models.MergeMethodSquash,
			},
		})

		result, err := rule.Evaluate(context.Background(), repo, dc)
		if err != nil {
			t.Fatalf("Evaluate error: %v", err)
		}

		if result.Status != rules.StatusPass {
			t.Fatalf("Status = %v, want PASS", result.Status)
		}

		if result.Metadata == nil {
			t.Fatal("expected metadata to be set")
		}

		if result.Metadata["baseline_source"] != string(models.BaselineSourceOrganizationRuleset) {
			t.Errorf("baseline_source = %v, want %v", result.Metadata["baseline_source"], models.BaselineSourceOrganizationRuleset)
		}
	})

	t.Run("FAIL result includes metadata", func(t *testing.T) {
		dc := data.NewMapDataContext(map[data.DependencyKey]any{
			data.DepRepoEffectiveMergeMethods: models.MergeMethodMerge | models.MergeMethodSquash,
			data.DepMergeBaseline: &models.MergeBaseline{
				State:   models.BaselineStateSet,
				Source:  models.BaselineSourceConvention,
				Allowed: models.MergeMethodSquash,
			},
		})

		result, err := rule.Evaluate(context.Background(), repo, dc)
		if err != nil {
			t.Fatalf("Evaluate error: %v", err)
		}

		if result.Status != rules.StatusFail {
			t.Fatalf("Status = %v, want FAIL", result.Status)
		}

		if result.Metadata == nil {
			t.Fatal("expected metadata to be set")
		}

		if result.Metadata["baseline_source"] != string(models.BaselineSourceConvention) {
			t.Errorf("baseline_source = %v, want %v", result.Metadata["baseline_source"], models.BaselineSourceConvention)
		}

		if result.Metadata["baseline_methods"] != "squash" {
			t.Errorf("baseline_methods = %v, want squash", result.Metadata["baseline_methods"])
		}

		if result.Metadata["effective_methods"] != "merge,squash" {
			t.Errorf("effective_methods = %v, want merge,squash", result.Metadata["effective_methods"])
		}
	})
}

func TestParseMergeMethodMask(t *testing.T) {
	tests := []struct {
		input   string
		want    models.MergeMethodMask
		wantErr bool
	}{
		{"", 0, false},
		{"merge", models.MergeMethodMerge, false},
		{"squash", models.MergeMethodSquash, false},
		{"rebase", models.MergeMethodRebase, false},
		{"merge,squash", models.MergeMethodMerge | models.MergeMethodSquash, false},
		{"MERGE,SQUASH", models.MergeMethodMerge | models.MergeMethodSquash, false},
		{" merge , squash ", models.MergeMethodMerge | models.MergeMethodSquash, false},
		{"merge,squash,rebase", models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase, false},
		{"invalid", 0, true},
		{"merge,invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseMergeMethodMask(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMergeMethodMask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseMergeMethodMask() = %v, want %v", got, tt.want)
			}
		})
	}
}
