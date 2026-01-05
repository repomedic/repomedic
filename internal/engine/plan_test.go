package engine

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"testing"

	"github.com/google/go-github/v66/github"
)

type mockRule struct {
	id   string
	deps []data.DependencyKey
}

func (r *mockRule) ID() string          { return r.id }
func (r *mockRule) Title() string       { return "Mock Rule" }
func (r *mockRule) Description() string { return "Mock" }
func (r *mockRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return r.deps, nil
}
func (r *mockRule) Evaluate(ctx context.Context, repo *github.Repository, data data.DataContext) (rules.Result, error) {
	return rules.Result{}, nil
}

func TestScanPlan_Deduplication(t *testing.T) {
	r1 := &mockRule{id: "r1", deps: []data.DependencyKey{"dep1", "dep2"}}
	r2 := &mockRule{id: "r2", deps: []data.DependencyKey{"dep2", "dep3"}}

	repo := RepositoryRef{
		ID:   1,
		Name: "test-repo",
		Repo: &github.Repository{ID: github.Int64(1)},
	}

	plan := NewScanPlan()
	err := plan.AddRepo(context.Background(), repo, []rules.Rule{r1, r2})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	rp := plan.RepoPlans[1]
	if len(rp.Dependencies) != 3 {
		t.Errorf("Expected 3 unique dependencies, got %d", len(rp.Dependencies))
	}

	if _, ok := rp.Dependencies["dep1"]; !ok {
		t.Error("Missing dep1")
	}
	if _, ok := rp.Dependencies["dep2"]; !ok {
		t.Error("Missing dep2")
	}
	if _, ok := rp.Dependencies["dep3"]; !ok {
		t.Error("Missing dep3")
	}
}

func TestSortedDependencies(t *testing.T) {
	rp := &RepoPlan{
		Dependencies: map[data.DependencyKey]data.DependencyRequest{
			data.DepRepoMetadata:                       {Key: data.DepRepoMetadata},
			data.DepRepoDefaultBranchClassicProtection: {Key: data.DepRepoDefaultBranchClassicProtection},
			data.DepRepoDefaultBranchEffectiveRules:    {Key: data.DepRepoDefaultBranchEffectiveRules},
		},
	}

	sorted := rp.SortedDependencies()

	if len(sorted) != 3 {
		t.Fatalf("Expected 3 sorted dependencies, got %d", len(sorted))
	}

	if sorted[0] != data.DepRepoMetadata {
		t.Errorf("Expected first dependency to be %s (P0), got %s", data.DepRepoMetadata, sorted[0])
	}

	// P1 items - order between them is alphabetical by key string
	// "repo.default_branch_protection" < "repo.default_branch_rules"
	if sorted[1] != data.DepRepoDefaultBranchClassicProtection {
		t.Errorf("Expected second dependency to be %s, got %s", data.DepRepoDefaultBranchClassicProtection, sorted[1])
	}
	if sorted[2] != data.DepRepoDefaultBranchEffectiveRules {
		t.Errorf("Expected third dependency to be %s, got %s", data.DepRepoDefaultBranchEffectiveRules, sorted[2])
	}
}

func TestScanPlan_AddRepo_FailsFastOnNilInputs(t *testing.T) {
	repo := RepositoryRef{
		ID:    1,
		Owner: "o",
		Name:  "n",
		Repo:  &github.Repository{ID: github.Int64(1)},
	}

	t.Run("nil context", func(t *testing.T) {
		plan := NewScanPlan()
		var nilCtx context.Context
		err := plan.AddRepo(nilCtx, repo, nil)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("uninitialized plan", func(t *testing.T) {
		plan := &ScanPlan{}
		err := plan.AddRepo(context.Background(), repo, nil)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
