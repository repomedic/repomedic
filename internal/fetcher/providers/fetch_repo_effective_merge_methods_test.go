package providers

import (
	"encoding/json"
	"testing"

	"repomedic/internal/data/models"

	"github.com/google/go-github/v66/github"
)

func TestApplyRulesetConstraints(t *testing.T) {
	targetRef := "refs/heads/main"
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	tests := []struct {
		name     string
		rulesets []*github.Ruleset
		want     models.MergeMethodMask
	}{
		{
			name:     "empty rulesets",
			rulesets: []*github.Ruleset{},
			want:     allMethods,
		},
		{
			name: "inactive ruleset is ignored",
			rulesets: []*github.Ruleset{
				{
					Enforcement: "disabled",
					Target:      github.String("branch"),
					Rules:       []*github.RepositoryRule{{Type: "required_linear_history"}},
				},
			},
			want: allMethods,
		},
		{
			name: "linear history removes merge",
			rulesets: []*github.Ruleset{
				{
					Enforcement: "active",
					Target:      github.String("branch"),
					Rules:       []*github.RepositoryRule{{Type: "required_linear_history"}},
				},
			},
			want: models.MergeMethodSquash | models.MergeMethodRebase,
		},
		{
			name: "tag ruleset is ignored",
			rulesets: []*github.Ruleset{
				{
					Enforcement: "active",
					Target:      github.String("tag"),
					Rules:       []*github.RepositoryRule{{Type: "required_linear_history"}},
				},
			},
			want: allMethods,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRulesetConstraints(tt.rulesets, targetRef, allMethods)
			if got != tt.want {
				t.Errorf("applyRulesetConstraints() = %v (%s), want %v (%s)",
					got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestApplyRulesConstraints(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	// Create merge queue params JSON.
	squashParams, _ := json.Marshal(github.MergeQueueRuleParameters{MergeMethod: "SQUASH"})

	tests := []struct {
		name  string
		rules []*github.RepositoryRule
		want  models.MergeMethodMask
	}{
		{
			name:  "empty rules",
			rules: []*github.RepositoryRule{},
			want:  allMethods,
		},
		{
			name:  "nil rules",
			rules: []*github.RepositoryRule{nil},
			want:  allMethods,
		},
		{
			name: "required_linear_history removes merge",
			rules: []*github.RepositoryRule{
				{Type: "required_linear_history"},
			},
			want: models.MergeMethodSquash | models.MergeMethodRebase,
		},
		{
			name: "merge_queue with squash",
			rules: []*github.RepositoryRule{
				{Type: "merge_queue", Parameters: (*json.RawMessage)(&squashParams)},
			},
			want: models.MergeMethodSquash,
		},
		{
			name: "combined linear history and merge queue",
			rules: []*github.RepositoryRule{
				{Type: "required_linear_history"},
				{Type: "merge_queue", Parameters: (*json.RawMessage)(&squashParams)},
			},
			want: models.MergeMethodSquash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRulesConstraints(tt.rules, allMethods)
			if got != tt.want {
				t.Errorf("applyRulesConstraints() = %v (%s), want %v (%s)",
					got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestApplyRulesConstraints_MergeQueueMethods(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase

	tests := []struct {
		name   string
		method string
		want   models.MergeMethodMask
	}{
		{"MERGE", "MERGE", models.MergeMethodMerge},
		{"SQUASH", "SQUASH", models.MergeMethodSquash},
		{"REBASE", "REBASE", models.MergeMethodRebase},
		{"lowercase merge", "merge", models.MergeMethodMerge},
		{"lowercase squash", "squash", models.MergeMethodSquash},
		{"lowercase rebase", "rebase", models.MergeMethodRebase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(github.MergeQueueRuleParameters{MergeMethod: tt.method})
			rules := []*github.RepositoryRule{
				{Type: "merge_queue", Parameters: (*json.RawMessage)(&params)},
			}

			got := applyRulesConstraints(rules, allMethods)
			if got != tt.want {
				t.Errorf("applyRulesConstraints() = %v (%s), want %v (%s)",
					got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestApplyRulesConstraints_ConflictingRules(t *testing.T) {
	// Merge queue requires MERGE, but linear history prohibits it.
	mergeParams, _ := json.Marshal(github.MergeQueueRuleParameters{MergeMethod: "MERGE"})

	rules := []*github.RepositoryRule{
		{Type: "merge_queue", Parameters: (*json.RawMessage)(&mergeParams)},
		{Type: "required_linear_history"},
	}

	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase
	got := applyRulesConstraints(rules, allMethods)

	// Result should be 0 (no valid methods).
	if got != 0 {
		t.Errorf("applyRulesConstraints() = %v (%s), want 0 (empty)",
			got, got.String())
	}
}
