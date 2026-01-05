package checks

import (
	"context"
	"repomedic/internal/rules"
	"testing"

	"repomedic/internal/data"
	"repomedic/internal/data/models"

	"github.com/google/go-github/v66/github"
)

func TestProtectedBranchesBlockDeletionRule_Evaluate(t *testing.T) {
	rule := &ProtectedBranchesBlockDeletionRule{}
	repo := &github.Repository{FullName: github.String("org/repo")}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name:           "missing dependency",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name: "nil dependency",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: nil,
			},
			expectedStatus: rules.StatusError,
		},
		{
			name: "wrong type",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: "nope",
			},
			expectedStatus: rules.StatusError,
		},
		{
			name: "truncated report",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{Truncated: true, Limit: 50},
			},
			expectedStatus: rules.StatusError,
		},
		{
			name: "no protected branches",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{Branches: nil, Truncated: false, Limit: 50},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "all protected branches block deletion",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{
					Branches: []models.ProtectedBranchDeletionStatus{
						{Name: "main", DeletionBlocked: true},
						{Name: "release", DeletionBlocked: true},
					},
					Limit: 50,
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "a protected branch allows deletion",
			data: map[data.DependencyKey]any{
				data.DepRepoProtectedBranchesDeletionStatus: &models.ProtectedBranchesDeletionStatus{
					Branches: []models.ProtectedBranchDeletionStatus{
						{Name: "main", DeletionBlocked: true},
						{Name: "release", DeletionBlocked: false},
					},
					Limit: 50,
				},
			},
			expectedStatus: rules.StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := data.NewMapDataContext(tt.data)
			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.Status != tt.expectedStatus {
				t.Fatalf("want %v, got %v (message=%q)", tt.expectedStatus, result.Status, result.Message)
			}
		})
	}
}
