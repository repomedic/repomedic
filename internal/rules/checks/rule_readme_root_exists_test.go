package checks

import (
	"context"
	"repomedic/internal/rules"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestReadmeRootExistsRule_Evaluate(t *testing.T) {
	rule := &ReadmeRootExistsRule{}
	repo := &github.Repository{FullName: github.String("acme/repo"), DefaultBranch: github.String("main")}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name: "PASS when found README.md",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: &models.ReadmePresence{Found: true, Path: "README.md"},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "PASS when found readme.md",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: &models.ReadmePresence{Found: true, Path: "readme.md"},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "PASS when found Readme.md",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: &models.ReadmePresence{Found: true, Path: "Readme.md"},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "FAIL when readme exists but is not README.md",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: &models.ReadmePresence{Found: true, Path: "README.rst"},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "FAIL when readme exists but not at root",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: &models.ReadmePresence{Found: true, Path: "docs/README.md"},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "FAIL when not found",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: &models.ReadmePresence{Found: false, Path: ""},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:           "ERROR when dependency missing",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name: "ERROR when wrong type",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: 123,
			},
			expectedStatus: rules.StatusError,
		},
		{
			name: "ERROR when nil",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchReadme: nil,
			},
			expectedStatus: rules.StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := data.NewMapDataContext(tt.data)
			res, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if res.Status != tt.expectedStatus {
				t.Fatalf("want %v, got %v", tt.expectedStatus, res.Status)
			}
		})
	}
}
