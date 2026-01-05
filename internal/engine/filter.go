package engine

import (
	"path"
	"repomedic/internal/config"
	"strings"
)

func FilterRepos(repos []RepositoryRef, cfg *config.Config) []RepositoryRef {
	if cfg == nil {
		panic("engine.FilterRepos: cfg must not be nil")
	}

	var filtered []RepositoryRef

	visibility := strings.TrimSpace(cfg.Targeting.Visibility)
	if visibility == "" {
		visibility = "all"
	}
	archivedPolicy := strings.TrimSpace(cfg.Targeting.Archived)
	if archivedPolicy == "" {
		archivedPolicy = "exclude"
	}
	forksPolicy := strings.TrimSpace(cfg.Targeting.Forks)
	if forksPolicy == "" {
		forksPolicy = "exclude"
	}

	requiredTopics := cfg.Targeting.Topic
	includePatterns := cfg.Targeting.Include
	excludePatterns := cfg.Targeting.Exclude

	for _, r := range repos {
		// Visibility
		if visibility != "all" {
			vis := repoVisibility(r)

			if visibility != vis {
				continue
			}
		}

		// Archived
		if archivedPolicy == "exclude" && r.Repo.GetArchived() {
			continue
		}
		if archivedPolicy == "only" && !r.Repo.GetArchived() {
			continue
		}

		// Forks
		if forksPolicy == "exclude" && r.Repo.GetFork() {
			continue
		}
		if forksPolicy == "only" && !r.Repo.GetFork() {
			continue
		}

		// Topics
		if len(requiredTopics) > 0 && !matchesAnyTopic(requiredTopics, r.Repo.Topics) {
			continue
		}

		// Include/exclude patterns (name matching)
		fullName := r.Repo.GetFullName()
		repoName := r.Repo.GetName()

		// If Include is set, must match at least one
		if len(includePatterns) > 0 && !matchesAnyPattern(includePatterns, fullName, repoName) {
			continue
		}

		// If Exclude is set, must not match any
		if len(excludePatterns) > 0 && matchesAnyPattern(excludePatterns, fullName, repoName) {
			continue
		}

		filtered = append(filtered, r)
	}

	// Max repos
	if cfg.Targeting.MaxRepos > 0 && len(filtered) > cfg.Targeting.MaxRepos {
		filtered = filtered[:cfg.Targeting.MaxRepos]
	}

	return filtered
}

func repoVisibility(r RepositoryRef) string {
	if v := strings.TrimSpace(r.Repo.GetVisibility()); v != "" {
		return v
	}
	if r.Repo.GetPrivate() {
		return "private"
	}
	return "public"
}

func matchesAnyTopic(requiredTopics, repoTopics []string) bool {
	if len(requiredTopics) == 0 {
		return true
	}

	for _, required := range requiredTopics {
		required = strings.TrimSpace(required)
		if required == "" {
			continue
		}
		for _, rt := range repoTopics {
			if required == rt {
				return true
			}
		}
	}
	return false
}

func matchesAnyPattern(patterns []string, fullName, repoName string) bool {
	for _, p := range patterns {
		if matchPattern(p, fullName, repoName) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, fullName, repoName string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	// If the pattern includes an owner component (contains '/'), match against full name.
	// Otherwise match against repo name only so patterns like "*-service" work with org scope.
	if strings.Contains(pattern, "/") {
		matched, _ := path.Match(pattern, fullName)
		return matched
	}
	matched, _ := path.Match(pattern, repoName)
	return matched
}
