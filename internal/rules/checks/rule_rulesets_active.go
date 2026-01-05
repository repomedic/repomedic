package checks

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"strings"

	"github.com/google/go-github/v66/github"
)

// RulesetsActiveRule checks that all rulesets (including inherited org rulesets)
// are in "active" enforcement mode. This rule fails if any ruleset is configured
// as "disabled" or "evaluate" (test mode).
type RulesetsActiveRule struct{}

func (r *RulesetsActiveRule) ID() string {
	return "rulesets-active"
}

func (r *RulesetsActiveRule) Title() string {
	return "All Rulesets Are Active"
}

func (r *RulesetsActiveRule) Description() string {
	return "Verifies that all rulesets (including inherited org rulesets) are in 'active' enforcement mode.\n\n" +
		"GitHub rulesets can be in one of three enforcement states: active, evaluate, or disabled. " +
		"This rule fails if any ruleset is configured as 'disabled' or 'evaluate', as these modes " +
		"indicate the rules are not being enforced."
}

func (r *RulesetsActiveRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoAllRulesets}, nil
}

func (r *RulesetsActiveRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	val, ok := dc.Get(data.DepRepoAllRulesets)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Dependency missing"), nil
	}

	// nil means no rulesets configured (known absence) - this passes since there are no inactive rulesets
	if val == nil {
		return rules.PassResultWithMessage(repo, r.ID(), "No rulesets configured"), nil
	}

	rulesets, ok := val.([]*github.Ruleset)
	if !ok {
		return rules.ErrorResult(repo, r.ID(), "Invalid dependency type"), nil
	}

	// No rulesets - pass
	if len(rulesets) == 0 {
		return rules.PassResultWithMessage(repo, r.ID(), "No rulesets configured"), nil
	}

	// Check each ruleset for non-active enforcement
	var inactiveRulesets []string
	activeCount := 0
	for _, rs := range rulesets {
		if rs == nil {
			continue
		}

		enforcement := strings.ToLower(strings.TrimSpace(rs.Enforcement))
		if enforcement != "active" {
			// Include the ruleset name and enforcement status in the message
			name := rs.Name
			if name == "" {
				name = fmt.Sprintf("id=%d", rs.GetID())
			}
			inactiveRulesets = append(inactiveRulesets, fmt.Sprintf("%s (%s)", name, enforcement))
		} else {
			activeCount++
		}
	}

	if len(inactiveRulesets) > 0 {
		msg := fmt.Sprintf("Found %d non-active ruleset(s): %s", len(inactiveRulesets), strings.Join(inactiveRulesets, ", "))
		return rules.FailResult(repo, r.ID(), msg), nil
	}

	return rules.PassResultWithMessage(repo, r.ID(), fmt.Sprintf("All %d ruleset(s) are active", activeCount)), nil
}

func init() {
	rules.Register(&RulesetsActiveRule{})
}
