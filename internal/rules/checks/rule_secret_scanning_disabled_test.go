package checks

import (
	"context"
	"testing"

	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v66/github"
)

func TestSecretScanningDisabledRule_Evaluate(t *testing.T) {
	tests := []struct {
		name           string
		repo           *github.Repository
		expectedStatus rules.Status
	}{
		{
			name: "missing security_and_analysis -> skipped",
			repo: &github.Repository{
				FullName: github.String("org/repo"),
			},
			expectedStatus: rules.StatusSkipped,
		},
		{
			name: "missing secret_scanning -> skipped",
			repo: &github.Repository{
				FullName:            github.String("org/repo"),
				SecurityAndAnalysis: &github.SecurityAndAnalysis{},
			},
			expectedStatus: rules.StatusSkipped,
		},
		{
			name: "enabled -> pass",
			repo: &github.Repository{
				FullName: github.String("org/repo"),
				SecurityAndAnalysis: &github.SecurityAndAnalysis{
					SecretScanning: &github.SecretScanning{
						Status: github.String("enabled"),
					},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "disabled -> fail",
			repo: &github.Repository{
				FullName: github.String("org/repo"),
				SecurityAndAnalysis: &github.SecurityAndAnalysis{
					SecretScanning: &github.SecretScanning{
						Status: github.String("disabled"),
					},
				},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "unknown status -> skipped",
			repo: &github.Repository{
				FullName: github.String("org/repo"),
				SecurityAndAnalysis: &github.SecurityAndAnalysis{
					SecretScanning: &github.SecretScanning{
						Status: github.String("unknown"),
					},
				},
			},
			expectedStatus: rules.StatusSkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &SecretScanningDisabledRule{}

			// We put the repo in the DataContext as DepRepoMetadata
			dc := data.NewMapDataContext(map[data.DependencyKey]any{
				data.DepRepoMetadata: tt.repo,
			})

			result, err := rule.Evaluate(context.Background(), tt.repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("want %v, got %v (message: %s)", tt.expectedStatus, result.Status, result.Message)
			}
		})
	}
}
