package rules

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/go-github/v81/github"
)

// AllowList handles common allow-listing logic for rules.
// It supports allowing by repository name (exact match), glob pattern, and topics.
type AllowList struct {
	Repos    map[string]bool
	Patterns []string
	Topics   []string
}

// Options returns the standard configuration options for allow-listing.
func (a *AllowList) Options() []Option {
	return []Option{
		{
			Name:        "allow.repos",
			Description: "Comma-separated list of allowed repositories (OWNER/REPO).",
		},
		{
			Name:        "allow.patterns",
			Description: "Comma-separated list of wildcard patterns for allowed repositories (e.g. acme/public-*, /docs-).",
		},
		{
			Name:        "allow.topics",
			Description: "Comma-separated list of topics. A repository with any of these topics is allowed.",
		},
	}
}

// Configure parses the configuration options to populate the AllowList.
func (a *AllowList) Configure(opts map[string]string) {
	a.Repos = make(map[string]bool)
	a.Patterns = nil
	a.Topics = nil

	if val, ok := opts["allow.repos"]; ok && val != "" {
		for _, s := range strings.Split(val, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				a.Repos[strings.ToLower(s)] = true
			}
		}
	}

	if val, ok := opts["allow.patterns"]; ok && val != "" {
		for _, s := range strings.Split(val, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				// We lowercase patterns to support case-insensitive matching
				a.Patterns = append(a.Patterns, strings.ToLower(s))
			}
		}
	}

	if val, ok := opts["allow.topics"]; ok && val != "" {
		for _, s := range strings.Split(val, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				a.Topics = append(a.Topics, strings.ToLower(s))
			}
		}
	}
}

// IsAllowed checks if the repository is allowed by any of the configured rules.
// It returns true and a reason string if allowed, otherwise false and empty string.
func (a *AllowList) IsAllowed(repo *github.Repository) (bool, string) {
	if repo == nil {
		return false, ""
	}

	fullName := strings.ToLower(repo.GetFullName())

	// Check allow.repos
	if a.Repos[fullName] {
		return true, "allow.repos"
	}

	// Check allow.patterns
	for _, pattern := range a.Patterns {
		if matched, _ := path.Match(pattern, fullName); matched {
			return true, "allow.patterns"
		}
	}

	// Check allow.topics
	if len(a.Topics) > 0 {
		for _, rt := range repo.Topics {
			rtLower := strings.ToLower(rt)
			for _, at := range a.Topics {
				if rtLower == at {
					return true, "allow.topics"
				}
			}
		}
	}

	return false, ""
}

// CheckResult evaluates the result and applies the allowlist logic.
// If the result is a failure and the repository is allowed, it converts the result to a pass.
func (a *AllowList) CheckResult(repo *github.Repository, result Result) Result {
	if result.Status == StatusFail {
		if allowed, reason := a.IsAllowed(repo); allowed {
			return PassResultWithMessage(repo, result.RuleID, fmt.Sprintf("Allowed failure: %s (Allowed by policy: %s)", result.Message, reason))
		}
	}
	return result
}
