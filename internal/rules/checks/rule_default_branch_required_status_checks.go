package checks

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strconv"
	"strings"

	"github.com/google/go-github/v81/github"
)

type DefaultBranchRequiredStatusChecks struct {
	minCount int
	allowAny bool
	required []string
}

func (r *DefaultBranchRequiredStatusChecks) ID() string {
	return "default-branch-required-status-checks"
}

func (r *DefaultBranchRequiredStatusChecks) Title() string {
	return "Default Branch Requires Status Checks"
}

func (r *DefaultBranchRequiredStatusChecks) Description() string {
	return "Verifies that merges to the default branch require passing status checks (classic protection or rulesets effective rules)."
}

func (r *DefaultBranchRequiredStatusChecks) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoDefaultBranchEffectiveRules,
	}, nil
}

func (r *DefaultBranchRequiredStatusChecks) Options() []rules.Option {
	return []rules.Option{
		{
			Name:        "min_count",
			Description: "Minimum number of required checks (if list is known)",
			Default:     "1",
		},
		{
			Name:        "allow_any",
			Description: "If true, any required checks are sufficient. If false, specific checks must be present.",
			Default:     "true",
		},
		{
			Name:        "required",
			Description: "Comma-separated list of required check names (only used if allow_any=false)",
			Default:     "",
		},
	}
}

func (r *DefaultBranchRequiredStatusChecks) Configure(opts map[string]string) error {
	// Defaults
	r.minCount = 1
	r.allowAny = true
	r.required = nil

	if val, ok := opts["min_count"]; ok {
		v, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid min_count: %w", err)
		}
		r.minCount = v
	}

	if val, ok := opts["allow_any"]; ok {
		v, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid allow_any: %w", err)
		}
		r.allowAny = v
	}

	if val, ok := opts["required"]; ok && val != "" {
		parts := strings.Split(val, ",")
		for _, p := range parts {
			r.required = append(r.required, strings.TrimSpace(p))
		}
	}

	return nil
}

func (r *DefaultBranchRequiredStatusChecks) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoDefaultBranchEffectiveRules)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("missing dependency: %s", data.DepRepoDefaultBranchEffectiveRules)), nil
	}

	branchRules, ok := val.(*github.BranchRules)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("unexpected type for %s: %T", data.DepRepoDefaultBranchEffectiveRules, val)), nil
	}

	// In v81, RequiredStatusChecks is a slice of *RequiredStatusChecksBranchRule
	if len(branchRules.RequiredStatusChecks) == 0 {
		return rules.FailResult(repo, r.ID(), "Default branch does not require status checks"), nil
	}

	// Use the first status check rule (typically there's only one)
	statusCheckRule := branchRules.RequiredStatusChecks[0]
	if statusCheckRule == nil {
		return rules.FailResult(repo, r.ID(), "Default branch does not require status checks"), nil
	}

	params := statusCheckRule.Parameters
	knownChecks := make([]string, 0, len(params.RequiredStatusChecks))
	for _, c := range params.RequiredStatusChecks {
		if c != nil {
			knownChecks = append(knownChecks, c.Context)
		}
	}

	// Metadata
	metadata := map[string]any{
		"branch":                repo.GetDefaultBranch(),
		"enforced":              true,
		"required_checks_count": len(knownChecks),
		"required_checks":       knownChecks,
		"source":                statusCheckRule.RulesetSource,
		"policy": map[string]any{
			"min_count": r.minCount,
			"allow_any": r.allowAny,
			"required":  r.required,
		},
	}
	if metadata["source"] == "" {
		metadata["source"] = "unknown"
	}

	if r.allowAny {
		// Check min_count
		if len(knownChecks) < r.minCount {
			return rules.FailResultWithMetadata(repo, r.ID(), fmt.Sprintf("Default branch requires %d status checks, but only %d are configured (min_count=%d)", len(knownChecks), len(knownChecks), r.minCount), metadata), nil
		}
		return rules.PassResultWithMetadata(repo, r.ID(), "Default branch requires status checks", metadata), nil
	}

	// allow_any = false, check specific requirements
	missing := []string{}
	for _, req := range r.required {
		found := false
		for _, known := range knownChecks {
			if strings.EqualFold(req, known) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return rules.FailResultWithMetadata(repo, r.ID(), fmt.Sprintf("Default branch is missing required status checks: %s", strings.Join(missing, ", ")), metadata), nil
	}

	return rules.PassResultWithMetadata(repo, r.ID(), "Default branch requires all specified status checks", metadata), nil
}

func init() {
	rules.Register(&DefaultBranchRequiredStatusChecks{})
}
