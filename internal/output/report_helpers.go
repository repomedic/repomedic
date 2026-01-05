package output

import (
	"fmt"
	"repomedic/internal/rules"
	"sort"
	"strings"
)

// Categories
const (
	CategoryDefaultBranchProtections = "Default branch allows merges without CI or enforced review gates"
	CategoryBranchDeletion           = "Admins can bypass branch protections or delete protected branches"
	CategoryRulesetEnforcement       = "Rulesets are not active or enforced"
	CategoryExposure                 = "Repository is publicly accessible or lacks secret scanning"
	CategoryRepoHygiene              = "Repository lacks standard documentation or ownership"
	CategoryNoBranchProtection       = "No Branch Protection Configured"
	CategoryOther                    = "Other"
)

var ruleCategories = map[string]string{
	"default-branch-protected":              CategoryDefaultBranchProtections,
	"default-branch-pr-required":            CategoryDefaultBranchProtections,
	"default-branch-pr-review-settings":     CategoryDefaultBranchProtections,
	"default-branch-required-status-checks": CategoryDefaultBranchProtections,
	"default-branch-no-force-push":          CategoryDefaultBranchProtections,
	"default-branch-restrict-push":          CategoryDefaultBranchProtections,

	"protected-branches-block-deletion": CategoryBranchDeletion,
	"branch-protect-enforce-admins":     CategoryBranchDeletion,

	"rulesets-active": CategoryRulesetEnforcement,

	"repo-visibility-public":   CategoryExposure,
	"secret-scanning-disabled": CategoryExposure,

	"codeowners-exists":  CategoryRepoHygiene,
	"readme-root-exists": CategoryRepoHygiene,
	"description-exists": CategoryRepoHygiene,

	"branch-protection-exists": CategoryNoBranchProtection,
}

var rulePriority = map[string]int{
	"default-branch-protected":              1,
	"default-branch-required-status-checks": 2,
	"default-branch-pr-required":            3,
	"default-branch-pr-review-settings":     4,
	"default-branch-no-force-push":          5,
	"default-branch-restrict-push":          6,
	"protected-branches-block-deletion":     7,
	"branch-protect-enforce-admins":         8,
	"repo-visibility-public":                9,
	"secret-scanning-disabled":              10,
}

var CategoryRiskDescription = map[string]string{
	CategoryDefaultBranchProtections: "Merges without required checks are a common root cause of production incidents and test bypasses.",
	CategoryBranchDeletion:           "Admins with write access can bypass protections, allowing force-pushes and history rewriting.",
	CategoryRulesetEnforcement:       "Inactive rulesets provide no protection against accidental or malicious changes.",
	CategoryExposure:                 "Public repositories without safeguards can expose intellectual property and secrets.",
	CategoryRepoHygiene:              "Lack of ownership and documentation delays incident response and increases bus factor.",
	CategoryNoBranchProtection:       "Unprotected repositories allow anyone with write access to push directly to the default branch.",
}

func getCategory(ruleID string) string {
	if cat, ok := ruleCategories[ruleID]; ok {
		return cat
	}
	return CategoryOther
}

func getPriority(ruleID string) int {
	if p, ok := rulePriority[ruleID]; ok {
		return p
	}
	return 999 // Low priority for hygiene/other
}

// normalizeErrorReason collapses whitespace, strips prefixes, and maps known patterns.
func normalizeErrorReason(errText string) string {
	s := strings.TrimSpace(errText)
	// Collapse whitespace
	fields := strings.Fields(s)
	s = strings.Join(fields, " ")

	// Strip common prefixes
	if idx := strings.Index(s, ": "); idx != -1 {
		// Heuristic: if the prefix looks like a rule ID or package path, strip it.
		// e.g. "repo.default_branch_protection: ..."
		prefix := s[:idx]
		if strings.Contains(prefix, ".") || strings.Contains(prefix, "_") {
			s = s[idx+2:]
		}
	}

	// Map known patterns
	if strings.Contains(s, "403 Forbidden") && (strings.Contains(s, "Upgrade to GitHub Pro") || strings.Contains(s, "feature requires GitHub Pro")) {
		return "403 Forbidden: feature requires GitHub Pro/Team"
	}

	// Fallback truncation
	if len(s) > 120 {
		return s[:117] + "..."
	}
	return s
}

type repoStats struct {
	Repo    string
	Pass    int
	Fail    int
	Skipped int
	Error   int
	Results []rules.Result
}

func (r *repoStats) KeyRisks() []string {
	// Key risks are derived from which specific rules failed for that repo.
	// Priority-ordered list:
	// “Default branch unprotected” (default-branch-protected)
	// “No required status checks” (default-branch-required-status-checks)
	// “Unexpected public” (repo-visibility-public)
	// “Secret scanning disabled” (secret-scanning-disabled)
	// “Admins can bypass protections” (branch-protect-enforce-admins)
	// “Protected branch deletable” (protected-branches-block-deletion)
	// “Rulesets not active” (rulesets-active)
	// “Missing CODEOWNERS” (codeowners-exists)

	var risks []string
	failedRules := make(map[string]bool)
	for _, res := range r.Results {
		if res.Status == rules.StatusFail {
			failedRules[res.RuleID] = true
		}
	}

	check := func(ruleID, label string) {
		if len(risks) >= 3 {
			return
		}
		if failedRules[ruleID] {
			risks = append(risks, label)
		}
	}

	check("default-branch-protected", "Default branch unprotected")
	check("default-branch-required-status-checks", "No required status checks")
	check("repo-visibility-public", "Unexpected public")
	check("secret-scanning-disabled", "Secret scanning disabled")
	check("branch-protect-enforce-admins", "Admins can bypass protections")
	check("protected-branches-block-deletion", "Protected branch deletable")
	check("rulesets-active", "Rulesets not active")
	check("codeowners-exists", "Missing CODEOWNERS")

	return risks
}

type categoryStats struct {
	Name           string
	ReposWithFail  int
	FailsByRule    map[string]int
	Representative []string // Top 3 rule IDs
}

func computeCategoryStats(results []rules.Result) []*categoryStats {
	stats := make(map[string]*categoryStats)
	repoFailsByCategory := make(map[string]map[string]bool) // category -> repo -> bool

	// Initialize known categories to ensure order (Priority Order)
	cats := []string{
		CategoryExposure,
		CategoryDefaultBranchProtections,
		CategoryBranchDeletion,
		CategoryRulesetEnforcement,
		CategoryNoBranchProtection,
		CategoryRepoHygiene,
	}
	for _, c := range cats {
		stats[c] = &categoryStats{
			Name:        c,
			FailsByRule: make(map[string]int),
		}
		repoFailsByCategory[c] = make(map[string]bool)
	}

	for _, r := range results {
		if r.Status != rules.StatusFail {
			continue
		}
		cat := getCategory(r.RuleID)
		if s, ok := stats[cat]; ok {
			s.FailsByRule[r.RuleID]++
			repoFailsByCategory[cat][r.Repo] = true
		}
	}

	var out []*categoryStats
	for _, c := range cats {
		s := stats[c]
		s.ReposWithFail = len(repoFailsByCategory[c])

		// Compute representative rules
		type ruleCount struct {
			ID    string
			Count int
		}
		var rc []ruleCount
		for id, count := range s.FailsByRule {
			rc = append(rc, ruleCount{id, count})
		}
		sort.Slice(rc, func(i, j int) bool {
			if rc[i].Count != rc[j].Count {
				return rc[i].Count > rc[j].Count
			}
			return rc[i].ID < rc[j].ID
		})

		limit := 3
		if len(rc) < limit {
			limit = len(rc)
		}
		for i := 0; i < limit; i++ {
			s.Representative = append(s.Representative, rc[i].ID)
		}

		if s.ReposWithFail > 0 {
			out = append(out, s)
		}
	}
	return out
}

type auditStats struct {
	FullyAudited     int
	PartiallyAudited int
	Blocked          int
	Blockers         []blockerInfo
}

type blockerInfo struct {
	Reason        string
	RepoCount     int
	ExampleRepos  []string
	ImpactedRules []string
}

func computeAuditStats(perRepo map[string]*repoStats) *auditStats {
	s := &auditStats{}

	// reason -> list of repos
	blockedGroups := make(map[string][]string)
	// reason -> ruleID -> count (for impacted rules in that group)
	groupImpactedRules := make(map[string]map[string]int)

	for _, rs := range perRepo {
		totalEvaluated := rs.Pass + rs.Fail + rs.Error
		if totalEvaluated == 0 {
			if rs.Error == 0 {
				s.FullyAudited++
			}
			continue
		}

		if rs.Error == 0 {
			s.FullyAudited++
			continue
		}

		isBlocked := float64(rs.Error)/float64(totalEvaluated) >= 0.5
		if !isBlocked {
			s.PartiallyAudited++
			continue
		}

		s.Blocked++

		// Find primary blocker reason for this repo
		reasons := make(map[string]int)
		for _, r := range rs.Results {
			if r.Status == rules.StatusError {
				reason := normalizeErrorReason(r.Message)
				reasons[reason]++
			}
		}

		primaryReason := "Unknown coverage blocker"
		maxCount := 0
		// Deterministic tie-breaking
		var sortedReasons []string
		for r := range reasons {
			sortedReasons = append(sortedReasons, r)
		}
		sort.Strings(sortedReasons)

		for _, r := range sortedReasons {
			if reasons[r] > maxCount {
				maxCount = reasons[r]
				primaryReason = r
			}
		}

		blockedGroups[primaryReason] = append(blockedGroups[primaryReason], rs.Repo)

		// Track impacted rules for this group
		if _, ok := groupImpactedRules[primaryReason]; !ok {
			groupImpactedRules[primaryReason] = make(map[string]int)
		}
		for _, r := range rs.Results {
			if r.Status == rules.StatusError {
				groupImpactedRules[primaryReason][r.RuleID]++
			}
		}
	}

	// Build s.Blockers
	for reason, repos := range blockedGroups {
		bi := blockerInfo{
			Reason:    reason,
			RepoCount: len(repos),
		}

		sort.Strings(repos)
		if len(repos) > 5 {
			bi.ExampleRepos = repos[:5]
		} else {
			bi.ExampleRepos = repos
		}

		// Impacted rules
		type rc struct {
			ID    string
			Count int
		}
		var rcs []rc
		for id, count := range groupImpactedRules[reason] {
			rcs = append(rcs, rc{id, count})
		}
		sort.Slice(rcs, func(i, j int) bool {
			if rcs[i].Count != rcs[j].Count {
				return rcs[i].Count > rcs[j].Count
			}
			return rcs[i].ID < rcs[j].ID
		})
		limit := 5
		if len(rcs) < limit {
			limit = len(rcs)
		}
		for i := 0; i < limit; i++ {
			bi.ImpactedRules = append(bi.ImpactedRules, rcs[i].ID)
		}

		s.Blockers = append(s.Blockers, bi)
	}

	sort.Slice(s.Blockers, func(i, j int) bool {
		if s.Blockers[i].RepoCount != s.Blockers[j].RepoCount {
			return s.Blockers[i].RepoCount > s.Blockers[j].RepoCount
		}
		return s.Blockers[i].Reason < s.Blockers[j].Reason
	})

	return s
}

func computeRiskScore(rs *repoStats) int {
	score := rs.Fail*10 + rs.Error*3

	hasFail := func(ruleID string) bool {
		for _, r := range rs.Results {
			if r.RuleID == ruleID && r.Status == rules.StatusFail {
				return true
			}
		}
		return false
	}

	if hasFail("repo-visibility-public") {
		score += 30
	}
	if hasFail("default-branch-protected") {
		score += 30
	}
	if hasFail("default-branch-required-status-checks") {
		score += 25
	}
	if hasFail("protected-branches-block-deletion") {
		score += 20
	}
	if hasFail("branch-protect-enforce-admins") {
		score += 15
	}
	if hasFail("secret-scanning-disabled") {
		score += 10
	}

	return score
}

func topRiskiestRepos(perRepo map[string]*repoStats, n int) []*repoStats {
	var all []*repoStats
	for _, rs := range perRepo {
		all = append(all, rs)
	}

	sort.Slice(all, func(i, j int) bool {
		s1 := computeRiskScore(all[i])
		s2 := computeRiskScore(all[j])
		if s1 != s2 {
			return s1 > s2
		}
		return all[i].Repo < all[j].Repo
	})

	if len(all) > n {
		return all[:n]
	}
	return all
}

func formatRepoList(repos []string, max int) string {
	if len(repos) == 0 {
		return ""
	}
	if len(repos) <= max {
		return fmt.Sprintf("%d repos (%s)", len(repos), strings.Join(repos, ", "))
	}
	return fmt.Sprintf("%d repos (%s, +%d more)", len(repos), strings.Join(repos[:max], ", "), len(repos)-max)
}
