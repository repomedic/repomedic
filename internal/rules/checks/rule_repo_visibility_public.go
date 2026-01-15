package checks

import (
	"context"

	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v81/github"
)

// RepoVisibilityPublicRule detects repositories whose visibility is PUBLIC but are not allow-listed.
type RepoVisibilityPublicRule struct{}

func init() {
	rules.Register(&RepoVisibilityPublicRule{})
}

func (r *RepoVisibilityPublicRule) ID() string {
	return "repo-visibility-public"
}

func (r *RepoVisibilityPublicRule) Title() string {
	return "Unexpected Public Repository"
}

func (r *RepoVisibilityPublicRule) Description() string {
	return "Verifies that repositories are not publicly visible unless explicitly allow-listed by policy."
}

func (r *RepoVisibilityPublicRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoMetadata,
	}, nil
}

func (r *RepoVisibilityPublicRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	// Use metadata from context if available, otherwise fallback to repo argument
	var targetRepo *github.Repository
	if val, ok := dc.Get(data.DepRepoMetadata); ok && val != nil {
		if meta, ok := val.(*github.Repository); ok {
			targetRepo = meta
		}
	}

	if targetRepo == nil {
		targetRepo = repo
	}

	if targetRepo == nil {
		return rules.ErrorResult(repo, r.ID(), "Repository metadata not available"), nil
	}

	// Check visibility
	isPublic := false
	if targetRepo.Visibility != nil {
		isPublic = *targetRepo.Visibility == "public"
	} else if targetRepo.Private != nil {
		isPublic = !*targetRepo.Private
	}

	if !isPublic {
		return rules.PassResultWithMessage(repo, r.ID(), "Repository is not public"), nil
	}

	return rules.FailResult(repo, r.ID(), "Repository is public"), nil
}
