package rules

import (
	"context"
	"repomedic/internal/data"

	"github.com/google/go-github/v81/github"
)

// AllowListWrapper wraps a Rule to provide automatic allowlist functionality.
type AllowListWrapper struct {
	Rule
	allowList AllowList
}

// ID returns the inner rule's ID.
func (w *AllowListWrapper) ID() string {
	return w.Rule.ID()
}

// Title returns the inner rule's Title.
func (w *AllowListWrapper) Title() string {
	return w.Rule.Title()
}

// Description returns the inner rule's Description.
func (w *AllowListWrapper) Description() string {
	return w.Rule.Description()
}

// Dependencies returns the inner rule's Dependencies.
func (w *AllowListWrapper) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return w.Rule.Dependencies(ctx, repo)
}

// Evaluate calls the inner rule's Evaluate and then applies the allowlist logic.
func (w *AllowListWrapper) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (Result, error) {
	result, err := w.Rule.Evaluate(ctx, repo, dc)
	if err != nil {
		return result, err
	}
	return w.allowList.CheckResult(repo, result), nil
}

// Options returns the combined options of the allowlist and the inner rule (if configurable).
func (w *AllowListWrapper) Options() []Option {
	opts := w.allowList.Options()
	if cr, ok := w.Rule.(ConfigurableRule); ok {
		opts = append(opts, cr.Options()...)
	}
	return opts
}

// Configure configures the allowlist and the inner rule (if configurable).
func (w *AllowListWrapper) Configure(opts map[string]string) error {
	w.allowList.Configure(opts)
	if cr, ok := w.Rule.(ConfigurableRule); ok {
		return cr.Configure(opts)
	}
	return nil
}
