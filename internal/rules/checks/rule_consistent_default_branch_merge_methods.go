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

// ConsistentDefaultBranchMergeMethodsRule checks that the repository's effective
// allowed merge methods for the default branch match the organization baseline.
//
// The baseline can be derived from:
// 1. An explicit required_configuration option (if set)
// 2. Organization rulesets (if any actively enforce merge method restrictions)
// 3. Convention from sampling existing repositories
//
// This rule helps ensure consistent merge policies across an organization.
type ConsistentDefaultBranchMergeMethodsRule struct {
	requiredConfiguration models.MergeMethodMask
	hasRequiredConfig     bool
}

func (r *ConsistentDefaultBranchMergeMethodsRule) ID() string {
	return "consistent-default-branch-merge-methods"
}

func (r *ConsistentDefaultBranchMergeMethodsRule) Title() string {
	return "Consistent Default Branch Merge Methods"
}

func (r *ConsistentDefaultBranchMergeMethodsRule) Description() string {
	return "Verifies that the repository's allowed merge methods for the default branch match the organization baseline.\n\n" +
		"The baseline can come from:\n" +
		"1. An explicit required_configuration option (comma-separated: merge,squash,rebase)\n" +
		"2. Organization rulesets that enforce merge method restrictions\n" +
		"3. Convention derived from sampling existing repositories\n\n" +
		"If no baseline can be determined (no org rulesets, no convention, no explicit config), the rule is skipped.\n" +
		"If the baseline is in conflict (incompatible policies detected), the rule fails."
}

func (r *ConsistentDefaultBranchMergeMethodsRule) Options() []rules.Option {
	return []rules.Option{
		{
			Name:        "required_configuration",
			Description: "Comma-separated list of allowed merge methods (merge,squash,rebase). When set, this becomes the baseline instead of using organization rulesets or convention.",
			Default:     "",
		},
	}
}

func (r *ConsistentDefaultBranchMergeMethodsRule) Configure(opts map[string]string) error {
	r.requiredConfiguration = 0
	r.hasRequiredConfig = false

	val, ok := opts["required_configuration"]
	if !ok || strings.TrimSpace(val) == "" {
		return nil
	}

	mask, err := parseMergeMethodMask(val)
	if err != nil {
		return fmt.Errorf("invalid required_configuration: %w", err)
	}

	if mask == 0 {
		return fmt.Errorf("required_configuration must specify at least one merge method")
	}

	r.requiredConfiguration = mask
	r.hasRequiredConfig = true
	return nil
}

// parseMergeMethodMask parses a comma-separated string of merge methods into a MergeMethodMask.
func parseMergeMethodMask(s string) (models.MergeMethodMask, error) {
	var mask models.MergeMethodMask

	parts := strings.Split(s, ",")
	for _, part := range parts {
		method := strings.TrimSpace(strings.ToLower(part))
		if method == "" {
			continue
		}
		switch method {
		case "merge":
			mask |= models.MergeMethodMerge
		case "squash":
			mask |= models.MergeMethodSquash
		case "rebase":
			mask |= models.MergeMethodRebase
		default:
			return 0, fmt.Errorf("unknown merge method: %q (valid: merge, squash, rebase)", method)
		}
	}

	return mask, nil
}

func (r *ConsistentDefaultBranchMergeMethodsRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	// Always need the repo's effective merge methods
	deps := []data.DependencyKey{data.DepRepoEffectiveMergeMethods}

	// If we have a required configuration, we don't need the org baseline
	if !r.hasRequiredConfig {
		deps = append(deps, data.DepMergeBaseline)
	}

	return deps, nil
}

func (r *ConsistentDefaultBranchMergeMethodsRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	// Get the repo's effective merge methods
	effectiveVal, ok := dc.Get(data.DepRepoEffectiveMergeMethods)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Dependency missing: repo effective merge methods"), nil
	}

	effectiveMask, ok := effectiveVal.(models.MergeMethodMask)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Invalid dependency type: repo effective merge methods"), nil
	}

	// Determine the baseline
	var baseline *models.MergeBaseline

	if r.hasRequiredConfig {
		// Use the explicit required configuration
		baseline = &models.MergeBaseline{
			State:   models.BaselineStateSet,
			Source:  models.BaselineSourceRequiredConfiguration,
			Allowed: r.requiredConfiguration,
		}
	} else {
		// Get the org-level baseline
		baselineVal, ok := dc.Get(data.DepMergeBaseline)
		if !ok {
			return rules.ErrorResult(repo, r.ID(), "Dependency missing: merge baseline"), nil
		}

		if baselineVal == nil {
			// nil means no baseline could be determined at all
			return rules.SkippedResult(repo, r.ID(), "No merge baseline available"), nil
		}

		baseline, ok = baselineVal.(*models.MergeBaseline)
		if !ok {
			return rules.ErrorResult(repo, r.ID(), "Invalid dependency type: merge baseline"), nil
		}
	}

	// Handle baseline states
	switch baseline.State {
	case models.BaselineStateNone:
		return rules.SkippedResult(repo, r.ID(), "No merge baseline could be determined"), nil

	case models.BaselineStateConflict:
		evidence := ""
		if len(baseline.Evidence) > 0 {
			evidence = fmt.Sprintf(" (%s)", strings.Join(baseline.Evidence, "; "))
		}
		return rules.FailResult(repo, r.ID(), fmt.Sprintf("Merge baseline conflict detected%s", evidence)), nil

	case models.BaselineStateSet:
		// Compare the effective mask against the baseline
		if effectiveMask == baseline.Allowed {
			msg := fmt.Sprintf("Merge methods match baseline (%s)", effectiveMask.String())
			return rules.PassResultWithMetadata(repo, r.ID(), msg, map[string]any{
				"baseline_source":   string(baseline.Source),
				"allowed_methods":   effectiveMask.String(),
				"effective_methods": effectiveMask.String(),
			}), nil
		}

		msg := fmt.Sprintf("Merge methods mismatch: repo allows %q but baseline expects %q",
			effectiveMask.String(), baseline.Allowed.String())
		return rules.FailResultWithMetadata(repo, r.ID(), msg, map[string]any{
			"baseline_source":   string(baseline.Source),
			"baseline_methods":  baseline.Allowed.String(),
			"effective_methods": effectiveMask.String(),
		}), nil

	default:
		return rules.ErrorResult(repo, r.ID(), fmt.Sprintf("Unknown baseline state: %s", baseline.State)), nil
	}
}

func init() {
	rules.Register(&ConsistentDefaultBranchMergeMethodsRule{})
}
