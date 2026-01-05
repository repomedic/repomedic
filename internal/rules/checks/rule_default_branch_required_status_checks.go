package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strconv"
	"strings"

	"github.com/google/go-github/v66/github"
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

type requiredStatusChecksParams struct {
	RequiredStatusChecks []struct {
		Context string `json:"context"`
	} `json:"required_status_checks"`
}

func (r *DefaultBranchRequiredStatusChecks) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoDefaultBranchEffectiveRules)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("missing dependency: %s", data.DepRepoDefaultBranchEffectiveRules)), nil
	}

	effectiveRules, ok := val.([]*github.RepositoryRule)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("unexpected type for %s: %T", data.DepRepoDefaultBranchEffectiveRules, val)), nil
	}

	var statusCheckRule *github.RepositoryRule
	for _, rule := range effectiveRules {
		if rule.Type == "required_status_checks" {
			statusCheckRule = rule
			break
		}
	}

	if statusCheckRule == nil {
		return rules.FailResult(repo, r.ID(), "Default branch does not require status checks"), nil
	}

	// Parse parameters
	var params requiredStatusChecksParams
	if statusCheckRule.Parameters != nil {
		if err := json.Unmarshal(*statusCheckRule.Parameters, &params); err != nil {
			// If we can't parse parameters, we treat it as an error because we can't verify the policy.
			return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("failed to parse rule parameters: %v", err)), nil
		}
	}

	knownChecks := make([]string, 0, len(params.RequiredStatusChecks))
	for _, c := range params.RequiredStatusChecks {
		knownChecks = append(knownChecks, c.Context)
	}

	// Metadata
	metadata := map[string]any{
		"branch":                repo.GetDefaultBranch(),
		"enforced":              true,
		"required_checks_count": len(knownChecks),
		"required_checks":       knownChecks,
		"source":                statusCheckRule.RulesetSource, // Might be empty or specific string
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
