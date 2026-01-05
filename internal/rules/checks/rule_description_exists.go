package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v66/github"
)

type DescriptionExistsRule struct{}

func (r *DescriptionExistsRule) ID() string {
	return "description-exists"
}

func (r *DescriptionExistsRule) Title() string {
	return "Repository Description Exists"
}

func (r *DescriptionExistsRule) Description() string {
	return "Verifies that the repository has a description."
}

func (r *DescriptionExistsRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoMetadata}, nil
}

func (r *DescriptionExistsRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoMetadata)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Dependency missing"), nil
	}

	meta, ok := val.(*github.Repository)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Unexpected type for metadata"), nil
	}

	if meta.GetDescription() == "" {
		return rules.FailResult(repo, r.ID(), "Repository description is empty"), nil
	}

	return rules.PassResult(repo, r.ID()), nil
}

func init() {
	rules.Register(&DescriptionExistsRule{})
}
