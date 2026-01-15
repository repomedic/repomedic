package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"testing"

	"github.com/google/go-github/v81/github"
)

func TestBranchProtectionExistsRule_Evaluate(t *testing.T) {
	repo := &github.Repository{
		Owner:         &github.User{Login: github.Ptr("test-org")},
		Name:          github.Ptr("test-repo"),
		FullName:      github.Ptr("test-org/test-repo"),
		DefaultBranch: github.Ptr("main"),
	}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name: "Pass - Classic branch protection configured",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{
					Branches: []models.ProtectedBranchDeletionStatus{
						{Name: "main", Source: "classic-branch-protection"},
					},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Ruleset configured",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{
					Branches: []models.ProtectedBranchDeletionStatus{
						{Name: "main", Source: "ruleset"},
					},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Multiple protected branches (classic and rulesets)",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{
					Branches: []models.ProtectedBranchDeletionStatus{
						{Name: "main", Source: "classic-branch-protection"},
						{Name: "release/*", Source: "ruleset"},
						{Name: "develop", Source: "ruleset"},
					},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Org-level ruleset protection",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{
					Branches: []models.ProtectedBranchDeletionStatus{
						{Name: "~DEFAULT_BRANCH", Source: "ruleset", Detail: "org-ruleset"},
					},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Fail - No protection at all (empty branches)",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{
					Branches: []models.ProtectedBranchDeletionStatus{},
				},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "Fail - No protection at all (nil value)",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: nil,
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:           "Error - Missing dependency",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name: "Error - Invalid dependency type",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: "not-a-status-object",
			},
			expectedStatus: rules.StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &BranchProtectionExistsRule{}

			dc := data.NewMapDataContext(tt.data)
			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate returned error: %v", err)
			}
			if result.Status != tt.expectedStatus {
				t.Fatalf("expected status %v, got %v (message: %s)", tt.expectedStatus, result.Status, result.Message)
			}
			if result.Repo != "test-org/test-repo" {
				t.Fatalf("expected repo %s, got %s", "test-org/test-repo", result.Repo)
			}
			if result.RuleID != rule.ID() {
				t.Fatalf("expected rule ID %s, got %s", rule.ID(), result.RuleID)
			}
		})
	}
}

func TestBranchProtectionExistsRule_Metadata(t *testing.T) {
	rule := &BranchProtectionExistsRule{}

	if rule.ID() != "branch-protection-exists" {
		t.Errorf("expected ID 'branch-protection-exists', got %s", rule.ID())
	}

	if rule.Title() == "" {
		t.Error("Title should not be empty")
	}

	if rule.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestBranchProtectionExistsRule_Dependencies(t *testing.T) {
	rule := &BranchProtectionExistsRule{}
	repo := &github.Repository{
		Owner:    &github.User{Login: github.Ptr("test-org")},
		Name:     github.Ptr("test-repo"),
		FullName: github.Ptr("test-org/test-repo"),
	}

	deps, err := rule.Dependencies(context.Background(), repo)
	if err != nil {
		t.Fatalf("Dependencies returned error: %v", err)
	}

	expectedDeps := []data.DependencyKey{
		data.DepRepoProtectedBranchesDeletionStatus,
	}

	if len(deps) != len(expectedDeps) {
		t.Fatalf("expected %d dependencies, got %d", len(expectedDeps), len(deps))
	}

	for i, dep := range deps {
		if dep != expectedDeps[i] {
			t.Errorf("expected dependency %s, got %s", expectedDeps[i], dep)
		}
	}
}
