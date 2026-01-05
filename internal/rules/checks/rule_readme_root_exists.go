package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"strings"

	"github.com/google/go-github/v66/github"
)

type ReadmeRootExistsRule struct{}

func (r *ReadmeRootExistsRule) ID() string {
	return "readme-root-exists"
}

func (r *ReadmeRootExistsRule) Title() string {
	return "README Exists at Repository Root"
}

func (r *ReadmeRootExistsRule) Description() string {
	return "Verifies that a README file exists at the repository root on the default branch. The README must be named README.md (case-insensitive)."
}

func (r *ReadmeRootExistsRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoDefaultBranchReadme}, nil
}

func (r *ReadmeRootExistsRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoDefaultBranchReadme)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Dependency missing"), nil
	}
	if val == nil {
		return rules.ErrorResult(repo, r.ID(), "Dependency is nil"), nil
	}

	presence, ok := val.(*models.ReadmePresence)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Invalid dependency type"), nil
	}

	if presence.Found {
		// Enforce README.md at repo root, but accept casing variants.
		p := strings.TrimSpace(presence.Path)
		if p != "" && strings.Contains(p, "/") {
			return rules.FailResult(repo, r.ID(), "README found but not at repository root (found "+p+")"), nil
		}
		if p == "" {
			return rules.FailResult(repo, r.ID(), "README found but path is unknown"), nil
		}
		if !strings.EqualFold(p, "README.md") {
			return rules.FailResult(repo, r.ID(), "README found but filename is not README.md (found "+p+")"), nil
		}
		return rules.PassResultWithMessage(repo, r.ID(), "README present at repository root ("+p+")"), nil
	}

	return rules.FailResult(repo, r.ID(), "README not found (expected README.md at repository root)"), nil
}

func init() {
	rules.Register(&ReadmeRootExistsRule{})
}
