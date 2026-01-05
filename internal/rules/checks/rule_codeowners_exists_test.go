package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestCodeownersExistsRule_Evaluate(t *testing.T) {
	repo := &github.Repository{FullName: github.String("acme/repo"), DefaultBranch: github.String("main")}

	tests := []struct {
		name           string
		configure      map[string]string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name:      "PASS either when root present",
			configure: map[string]string{"location": "either"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: &models.CodeownersPresence{Root: true, GitHub: false},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name:      "PASS either when github present",
			configure: map[string]string{"location": "either"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: &models.CodeownersPresence{Root: false, GitHub: true},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name:      "FAIL either when none present",
			configure: map[string]string{"location": "either"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: &models.CodeownersPresence{Root: false, GitHub: false},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:      "PASS root when root present",
			configure: map[string]string{"location": "root"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: &models.CodeownersPresence{Root: true, GitHub: true},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name:      "FAIL root when only github present",
			configure: map[string]string{"location": "root"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: &models.CodeownersPresence{Root: false, GitHub: true},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:      "PASS github when github present",
			configure: map[string]string{"location": "github"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: &models.CodeownersPresence{Root: true, GitHub: true},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name:      "FAIL github when only root present",
			configure: map[string]string{"location": "github"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: &models.CodeownersPresence{Root: true, GitHub: false},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:           "ERROR when dependency missing",
			configure:      map[string]string{"location": "either"},
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name:      "ERROR when dependency wrong type",
			configure: map[string]string{"location": "either"},
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchCodeowners: 123,
			},
			expectedStatus: rules.StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &CodeownersExistsRule{}
			if err := rule.Configure(tt.configure); err != nil {
				t.Fatalf("Configure error: %v", err)
			}

			dc := data.NewMapDataContext(tt.data)
			res, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if res.Status != tt.expectedStatus {
				t.Fatalf("want %v, got %v", tt.expectedStatus, res.Status)
			}
			if tt.name == "PASS allowed repo" {
				expectedMsg := "Allowed failure: CODEOWNERS not found at CODEOWNERS or .github/CODEOWNERS (Allowed by policy: allow.repos)"
				if res.Message != expectedMsg {
					t.Errorf("want message %q, got %q", expectedMsg, res.Message)
				}
			}
		})
	}
}

func TestCodeownersExistsRule_Configure_InvalidLocation(t *testing.T) {
	rule := &CodeownersExistsRule{}
	if err := rule.Configure(map[string]string{"location": "somewhere"}); err == nil {
		t.Fatalf("expected error")
	}
}
