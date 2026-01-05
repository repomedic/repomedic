package engine

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"sort"
)

type ScanPlan struct {
	RepoPlans map[int64]*RepoPlan
}

type RepoPlan struct {
	Repo         RepositoryRef
	Dependencies map[data.DependencyKey]data.DependencyRequest
	Rules        []rules.Rule
}

func NewScanPlan() *ScanPlan {
	return &ScanPlan{
		RepoPlans: make(map[int64]*RepoPlan),
	}
}

func (p *ScanPlan) AddRepo(ctx context.Context, repo RepositoryRef, selectedRules []rules.Rule) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	if p == nil {
		return fmt.Errorf("scan plan is nil")
	}
	if p.RepoPlans == nil {
		return fmt.Errorf("scan plan is not initialized (RepoPlans is nil); use NewScanPlan")
	}
	if repo.Repo == nil {
		return fmt.Errorf("repo object is nil for %s/%s (id=%d)", repo.Owner, repo.Name, repo.ID)
	}

	rp := &RepoPlan{
		Repo:         repo,
		Dependencies: make(map[data.DependencyKey]data.DependencyRequest),
		Rules:        selectedRules,
	}

	for _, r := range selectedRules {
		deps, err := r.Dependencies(ctx, repo.Repo)
		if err != nil {
			return fmt.Errorf("failed to get dependencies for rule %s: %w", r.ID(), err)
		}

		for _, d := range deps {
			// Simple deduplication by key.
			// TODO: Handle params merging if needed.
			if _, exists := rp.Dependencies[d]; !exists {
				rp.Dependencies[d] = data.DependencyRequest{Key: d}
			}
		}
	}

	p.RepoPlans[repo.ID] = rp
	return nil
}

// SortedDependencies returns the list of dependency keys sorted by priority (P0 first).
func (rp *RepoPlan) SortedDependencies() []data.DependencyKey {
	keys := make([]data.DependencyKey, 0, len(rp.Dependencies))
	for k := range rp.Dependencies {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		p1 := data.Priority(keys[i])
		p2 := data.Priority(keys[j])
		if p1 != p2 {
			return p1 < p2
		}
		return keys[i] < keys[j] // Stable sort for same priority
	})

	return keys
}
