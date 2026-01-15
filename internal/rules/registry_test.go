package rules

import (
	"context"
	"repomedic/internal/data"
	"testing"

	"github.com/google/go-github/v81/github"
)

type dummyRule struct {
	id string
}

func (r *dummyRule) ID() string          { return r.id }
func (r *dummyRule) Title() string       { return "Dummy Rule" }
func (r *dummyRule) Description() string { return "Does nothing" }
func (r *dummyRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return nil, nil
}
func (r *dummyRule) Evaluate(ctx context.Context, repo *github.Repository, data data.DataContext) (Result, error) {
	return Result{}, nil
}

func TestRegistry(t *testing.T) {
	// Clear registry for test
	mu.Lock()
	registry = make(map[string]Rule)
	mu.Unlock()

	r1 := &dummyRule{id: "rule1"}
	r2 := &dummyRule{id: "rule2"}

	Register(r1)
	Register(r2)

	// Test List
	all := List()
	if len(all) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(all))
	}

	// Test Resolve
	selected, err := Resolve("rule1")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(selected) != 1 || selected[0].ID() != "rule1" {
		t.Errorf("Expected rule1, got %v", selected)
	}

	// Test Resolve All
	selected, err = Resolve("")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(selected) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(selected))
	}

	// Test Resolve Unknown
	_, err = Resolve("unknown")
	if err == nil {
		t.Error("Expected error for unknown rule")
	}
}
