package rules

import (
	"context"
	"repomedic/internal/data"

	"github.com/google/go-github/v66/github"
)

type Rule interface {
	ID() string
	Title() string
	Description() string

	// Dependencies declares required GitHub data for this repo.
	Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error)

	// Evaluate runs rule logic using only DataContext.
	// Rules MUST NOT call GitHub APIs.
	Evaluate(ctx context.Context, repo *github.Repository, data data.DataContext) (Result, error)
}

type Option struct {
	Name        string
	Description string
	Default     string
}

type ConfigurableRule interface {
	Rule
	Options() []Option
	Configure(opts map[string]string) error
}
