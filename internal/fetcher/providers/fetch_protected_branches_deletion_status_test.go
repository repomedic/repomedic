package providers

import (
	"testing"

	"repomedic/internal/data/models"
)

func TestDeduplicateScopes(t *testing.T) {
	tests := []struct {
		name     string
		input    []models.ProtectedBranchDeletionStatus
		expected []models.ProtectedBranchDeletionStatus
	}{
		{
			name:     "empty input",
			input:    []models.ProtectedBranchDeletionStatus{},
			expected: []models.ProtectedBranchDeletionStatus{},
		},
		{
			name: "single scope",
			input: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: true, Source: "classic-branch-protection", Detail: "allowsDeletions=false"},
			},
			expected: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: true, Source: "classic-branch-protection", Detail: "allowsDeletions=false"},
			},
		},
		{
			name: "duplicate pattern both block deletion",
			input: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: true, Source: "classic-branch-protection", Detail: "allowsDeletions=false"},
				{Name: "refs/heads/main", DeletionBlocked: true, Source: "ruleset:MyRuleset", Detail: "id=123"},
			},
			expected: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: true, Source: "classic-branch-protection, ruleset:MyRuleset", Detail: "allowsDeletions=false; id=123"},
			},
		},
		{
			name: "duplicate pattern one allows deletion",
			input: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: false, Source: "classic-branch-protection", Detail: "allowsDeletions=true"},
				{Name: "refs/heads/main", DeletionBlocked: true, Source: "ruleset:MyRuleset", Detail: "id=123"},
			},
			expected: []models.ProtectedBranchDeletionStatus{
				// Should be blocked because ANY source blocking means blocked.
				{Name: "main", DeletionBlocked: true, Source: "classic-branch-protection, ruleset:MyRuleset", Detail: "allowsDeletions=true; id=123"},
			},
		},
		{
			name: "duplicate pattern neither blocks deletion",
			input: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: false, Source: "classic-branch-protection", Detail: "allowsDeletions=true"},
				{Name: "refs/heads/main", DeletionBlocked: false, Source: "ruleset:MyRuleset", Detail: "id=123"},
			},
			expected: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: false, Source: "classic-branch-protection, ruleset:MyRuleset", Detail: "allowsDeletions=true; id=123"},
			},
		},
		{
			name: "multiple different patterns",
			input: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: true, Source: "classic-branch-protection", Detail: "allowsDeletions=false"},
				{Name: "release/*", DeletionBlocked: true, Source: "classic-branch-protection", Detail: "allowsDeletions=false"},
				{Name: "refs/heads/develop", DeletionBlocked: false, Source: "ruleset:DevRuleset", Detail: "id=456"},
			},
			expected: []models.ProtectedBranchDeletionStatus{
				{Name: "main", DeletionBlocked: true, Source: "classic-branch-protection", Detail: "allowsDeletions=false"},
				{Name: "release/*", DeletionBlocked: true, Source: "classic-branch-protection", Detail: "allowsDeletions=false"},
				{Name: "develop", DeletionBlocked: false, Source: "ruleset:DevRuleset", Detail: "id=456"},
			},
		},
		{
			name: "preserves insertion order",
			input: []models.ProtectedBranchDeletionStatus{
				{Name: "refs/heads/develop", DeletionBlocked: true, Source: "ruleset:A", Detail: ""},
				{Name: "main", DeletionBlocked: true, Source: "classic", Detail: ""},
				{Name: "release/*", DeletionBlocked: true, Source: "classic", Detail: ""},
			},
			expected: []models.ProtectedBranchDeletionStatus{
				{Name: "develop", DeletionBlocked: true, Source: "ruleset:A", Detail: ""},
				{Name: "main", DeletionBlocked: true, Source: "classic", Detail: ""},
				{Name: "release/*", DeletionBlocked: true, Source: "classic", Detail: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateScopes(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(result), len(tt.expected))
			}
			for i := range result {
				if result[i].Name != tt.expected[i].Name {
					t.Errorf("item %d Name: got %q, want %q", i, result[i].Name, tt.expected[i].Name)
				}
				if result[i].DeletionBlocked != tt.expected[i].DeletionBlocked {
					t.Errorf("item %d DeletionBlocked: got %v, want %v", i, result[i].DeletionBlocked, tt.expected[i].DeletionBlocked)
				}
			}
		})
	}
}
