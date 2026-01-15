package checks

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/rules"
	"strings"

	"github.com/google/go-github/v81/github"
)

type CodeownersExistsRule struct {
	location string
}

func (r *CodeownersExistsRule) ID() string {
	return "codeowners-exists"
}

func (r *CodeownersExistsRule) Title() string {
	return "CODEOWNERS File Exists"
}

func (r *CodeownersExistsRule) Description() string {
	return "Verifies that a CODEOWNERS file exists on the repository's default branch.\n\n" +
		"Accepted locations:\n" +
		"- CODEOWNERS\n" +
		"- .github/CODEOWNERS\n\n" +
		"Options:\n" +
		"- location: where CODEOWNERS must exist (either|root|github)\n\n" +
		"Examples:\n" +
		"  # Default: accept either location\n" +
		"  repomedic scan --repos org/repo --rules codeowners-exists\n\n" +
		"  # Require CODEOWNERS at the repo root\n" +
		"  repomedic scan --repos org/repo --rules codeowners-exists --set codeowners-exists.location=root\n\n" +
		"  # Require CODEOWNERS in the .github directory\n" +
		"  repomedic scan --repos org/repo --rules codeowners-exists --set codeowners-exists.location=github"
}

func (r *CodeownersExistsRule) Options() []rules.Option {
	return []rules.Option{
		{
			Name:        "location",
			Description: "Where CODEOWNERS must exist on the default branch: either (default), root (CODEOWNERS), or github (.github/CODEOWNERS).",
			Default:     "either",
		},
	}
}

func (r *CodeownersExistsRule) Configure(opts map[string]string) error {
	r.location = "either"

	if v, ok := opts["location"]; ok {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" {
			r.location = v
		}
	}

	switch r.location {
	case "either", "root", "github":
		return nil
	case ".github":
		r.location = "github"
		return nil
	default:
		return fmt.Errorf("invalid value for location: %q (must be either|root|github)", r.location)
	}
}

func (r *CodeownersExistsRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoDefaultBranchCodeowners}, nil
}

func (r *CodeownersExistsRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoDefaultBranchCodeowners)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Dependency missing"), nil
	}
	if val == nil {
		return rules.ErrorResult(repo, r.ID(), "Dependency is nil"), nil
	}

	presence, ok := val.(*models.CodeownersPresence)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Invalid dependency type"), nil
	}

	var result rules.Result
	switch r.location {
	case "root":
		if presence.Root {
			result = rules.PassResultWithMessage(repo, r.ID(), "CODEOWNERS present at repository root")
		} else {
			result = rules.FailResult(repo, r.ID(), "CODEOWNERS not found at repository root")
		}
	case "github":
		if presence.GitHub {
			result = rules.PassResultWithMessage(repo, r.ID(), "CODEOWNERS present in .github directory")
		} else {
			result = rules.FailResult(repo, r.ID(), "CODEOWNERS not found in .github directory")
		}
	case "either", "":
		if presence.Root || presence.GitHub {
			loc := "repository root"
			if presence.GitHub {
				loc = ".github directory"
			}
			if presence.Root && presence.GitHub {
				loc = "repository root and .github directory"
			}
			result = rules.PassResultWithMessage(repo, r.ID(), "CODEOWNERS present in "+loc)
		} else {
			result = rules.FailResult(repo, r.ID(), "CODEOWNERS not found at CODEOWNERS or .github/CODEOWNERS")
		}
	default:
		// Defensive: should be prevented by Configure.
		return rules.ErrorResult(repo, r.ID(), "Invalid configuration"), nil
	}

	return result, nil
}

func init() {
	rules.Register(&CodeownersExistsRule{})
}
