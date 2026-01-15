package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"testing"

	"github.com/google/go-github/v81/github"
)

func TestBranchProtectEnforceAdmins_Evaluate(t *testing.T) {
	repo := &github.Repository{FullName: github.Ptr("org/repo")}

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
			name: "wrong type",
			data: map[data.DependencyKey]any{
				data.DepRepoClassicBranchProtections: "not a protections object",
			},
			expectedStatus: rules.StatusError,
		},
		{
			name: "no protections",
			data: map[data.DependencyKey]any{
				data.DepRepoClassicBranchProtections: &models.ClassicBranchProtections{
					Protections: []models.ClassicBranchProtection{},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "all enforced",
			data: map[data.DependencyKey]any{
				data.DepRepoClassicBranchProtections: &models.ClassicBranchProtections{
					Protections: []models.ClassicBranchProtection{
						{Pattern: "main", IsAdminEnforced: true},
						{Pattern: "release/*", IsAdminEnforced: true},
					},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "one not enforced",
			data: map[data.DependencyKey]any{
				data.DepRepoClassicBranchProtections: &models.ClassicBranchProtections{
					Protections: []models.ClassicBranchProtection{
						{Pattern: "main", IsAdminEnforced: true},
						{Pattern: "release/*", IsAdminEnforced: false},
					},
				},
			},
			expectedStatus: rules.StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &BranchProtectEnforceAdmins{}

			dc := data.NewMapDataContext(tt.data)
			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("want %v, got %v (message: %s)", tt.expectedStatus, result.Status, result.Message)
			}
			if tt.name == "no protections" {
				expectedMsg := "No classic branch protection rules found, so there are no admin enforcement settings to check"
				if result.Message != expectedMsg {
					t.Errorf("want message %q, got %q", expectedMsg, result.Message)
				}
			}
			if tt.name == "allowed repo" {
				expectedMsg := "Allowed failure: Enforce admins is disabled for classic branch protection rules: main (Allowed by policy: allow.repos)"
				if result.Message != expectedMsg {
					t.Errorf("want message %q, got %q", expectedMsg, result.Message)
				}
			}
		})
	}
}
