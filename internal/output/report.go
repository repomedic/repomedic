package output

import (
	"fmt"
	"os"
	"repomedic/internal/rules"
	"sort"
	"strings"
	"sync"
)

type ReportSink struct {
	path         string
	file         *os.File
	mu           sync.Mutex
	results      []rules.Result
	repos        map[string]struct{}
	exitCode     int
	haveExitCode bool
}

func NewReportSink(path string) (*ReportSink, error) {
	if path == "" {
		return nil, fmt.Errorf("report path required")
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create report file: %w", err)
	}

	return &ReportSink{
		path:  path,
		file:  f,
		repos: make(map[string]struct{}),
	}, nil
}

func (s *ReportSink) Write(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch t := v.(type) {
	case rules.Result:
		s.results = append(s.results, t)
		if t.Repo != "" {
			s.repos[t.Repo] = struct{}{}
		}
	case Event:
		if t.Repo != "" {
			s.repos[t.Repo] = struct{}{}
		}
		if t.Type == "run.finished" {
			s.exitCode = t.ExitCode
			s.haveExitCode = true
		}
	}
	return nil
}

func (s *ReportSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	writeErr := func(err error) error {
		_ = s.file.Close()
		return err
	}

	// Deterministic repo list (collected from both lifecycle events and results via Write()).
	var repos []string
	for repo := range s.repos {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	// 1. Aggregate Data
	perRepo := make(map[string]*repoStats)
	for _, repo := range repos {
		perRepo[repo] = &repoStats{Repo: repo}
	}

	uniqueRules := make(map[string]struct{})
	var fails, skips, errs []rules.Result

	for _, r := range s.results {
		if r.RuleID != "" {
			uniqueRules[r.RuleID] = struct{}{}
		}
		if r.Repo != "" {
			if _, ok := perRepo[r.Repo]; !ok {
				perRepo[r.Repo] = &repoStats{Repo: r.Repo}
			}
			rs := perRepo[r.Repo]
			rs.Results = append(rs.Results, r)
			switch r.Status {
			case rules.StatusPass:
				rs.Pass++
			case rules.StatusFail:
				rs.Fail++
			case rules.StatusSkipped:
				rs.Skipped++
			case rules.StatusError:
				rs.Error++
			}
		}

		switch r.Status {
		case rules.StatusFail:
			fails = append(fails, r)
		case rules.StatusSkipped:
			skips = append(skips, r)
		case rules.StatusError:
			errs = append(errs, r)
		}
	}

	catStats := computeCategoryStats(s.results)
	audit := computeAuditStats(perRepo)

	// 2. Build Report
	var b strings.Builder
	b.WriteString("# RepoMedic Scan Report\n\n")

	// --- Executive Risk Brief ---
	// Calculate stats for the brief
	// 1. Public and not allow-listed
	publicFail := 0
	for _, r := range s.results {
		if r.RuleID == "repo-visibility-public" && r.Status == rules.StatusFail {
			publicFail++
		}
	}

	// 2. Default branch not safe
	unsafeDefaultBranch := 0
	for _, rs := range perRepo {
		isUnsafe := false
		for _, r := range rs.Results {
			if r.Status == rules.StatusFail {
				if getCategory(r.RuleID) == CategoryDefaultBranchProtections ||
					r.RuleID == "default-branch-protected" ||
					r.RuleID == "default-branch-required-status-checks" {
					isUnsafe = true
					break
				}
			}
		}
		if isUnsafe {
			unsafeDefaultBranch++
		}
	}

	// 3. Blocked due to 403
	blocked403 := 0
	for _, bInfo := range audit.Blockers {
		if strings.Contains(bInfo.Reason, "403 Forbidden") {
			blocked403 += bInfo.RepoCount
		}
	}

	b.WriteString("### 🚨 Executive Risk Brief\n\n")

	b.WriteString("**What RepoMedic found**\n")
	if publicFail > 0 {
		b.WriteString(fmt.Sprintf("- **%d repos are publicly visible and not allow-listed.**\n", publicFail))
	}
	if unsafeDefaultBranch > 0 {
		b.WriteString(fmt.Sprintf("- **%d repos allow merges to the default branch without required status checks/protection.**\n", unsafeDefaultBranch))
	}
	if blocked403 > 0 {
		b.WriteString(fmt.Sprintf("- **%d repos could not be audited for protections due to plan/permissions (403).**\n", blocked403))
	}
	if publicFail == 0 && unsafeDefaultBranch == 0 && blocked403 == 0 {
		b.WriteString("- No critical high-level risks found.\n")
	}

	b.WriteString("\n**Why this matters**\n")
	if unsafeDefaultBranch > 0 {
		b.WriteString("- Unprotected default branches allow direct pushes that can bypass CI/CD and peer review.\n")
	}
	if publicFail > 0 {
		b.WriteString("- Public repositories without strict controls increase the risk of accidental secret exposure.\n")
	}
	if blocked403 > 0 {
		b.WriteString("- Blind spots in audit coverage prevent accurate risk assessment and can hide drift.\n")
	}
	if publicFail == 0 && unsafeDefaultBranch == 0 && blocked403 == 0 {
		b.WriteString("- Continuous monitoring ensures security posture remains strong.\n")
	}

	b.WriteString("\n**What to do first**\n")
	if unsafeDefaultBranch > 0 {
		b.WriteString("- Lock down default branches on high-risk repositories to prevent direct pushes and enforce review.\n")
	}
	if publicFail > 0 {
		b.WriteString("- Restrict public repository access or implement strict allow-listing to prevent data exposure.\n")
	}
	if blocked403 > 0 {
		b.WriteString("- Audit permissions and resolve blockers to ensure complete visibility into repository protections.\n")
	}
	if publicFail == 0 && unsafeDefaultBranch == 0 && blocked403 == 0 {
		if len(fails) > 0 {
			b.WriteString("- Review and remediate critical findings starting with the most exposed repositories.\n")
		} else {
			b.WriteString("- No immediate actions required.\n")
		}
	}
	b.WriteString("\n")

	// Top Risk Areas
	b.WriteString("### Top Risk Areas\n")
	b.WriteString("Baseline = minimum expected controls: branch protection enabled and default-branch merges gated by PRs + required status checks.\n\n")
	if len(catStats) == 0 {
		b.WriteString("- No top risk areas found.\n")
	} else {
		hasRisks := false
		for _, cs := range catStats {
			// Do NOT include Repo Hygiene in Top Risk Areas unless it is the only failing category.
			if cs.Name == CategoryRepoHygiene && len(catStats) > 1 {
				continue
			}
			hasRisks = true

			// Get example repos
			var examples []string
			for _, repo := range repos {
				rs := perRepo[repo]
				hasFail := false
				for _, r := range rs.Results {
					if r.Status == rules.StatusFail && getCategory(r.RuleID) == cs.Name {
						hasFail = true
						break
					}
				}
				if hasFail {
					examples = append(examples, repo)
				}
			}

			impact := "impact unknown"
			switch cs.Name {
			case CategoryDefaultBranchProtections:
				impact = "can bypass CI/CD & review"
			case CategoryBranchDeletion:
				impact = "can compromise audit history"
			case CategoryRulesetEnforcement:
				impact = "protections not active"
			case CategoryExposure:
				impact = "can expose IP/secrets"
			case CategoryRepoHygiene:
				impact = "delays incident response"
			case CategoryNoBranchProtection:
				impact = "direct push to the default branch"
			}

			// Name | Repo count | Impact phrase | Example repos
			// Using formatRepoList for "Repo count ... Example repos" part, but need to inject impact.
			// formatRepoList returns "N repos (a, b, ...)"
			// We want: Name: N repos - Impact - (a, b, ...)

			// Let's reconstruct manually to match prompt exactly
			countStr := fmt.Sprintf("%d repos", len(examples))
			exStr := ""
			if len(examples) > 0 {
				if len(examples) <= 3 {
					exStr = fmt.Sprintf("(%s)", strings.Join(examples, ", "))
				} else {
					exStr = fmt.Sprintf("(%s, +%d more)", strings.Join(examples[:3], ", "), len(examples)-3)
				}
			}

			b.WriteString(fmt.Sprintf("- **%s**: %s - %s %s\n", cs.Name, countStr, impact, exStr))
		}
		if !hasRisks {
			b.WriteString("- No top risk areas found (only hygiene issues).\n")
		}
	}
	b.WriteString("\n")

	// --- Top Findings ---
	b.WriteString("## Controls Failing Across the Fleet\n\n")
	if len(catStats) == 0 {
		b.WriteString("No findings.\n\n")
	} else {
		b.WriteString("| Category | Repos | Representative Rules |\n")
		b.WriteString("| --- | ---: | --- |\n")
		for _, cs := range catStats {
			desc := CategoryRiskDescription[cs.Name]
			nameWithDesc := fmt.Sprintf("**%s**<br>_%s_", cs.Name, desc)
			b.WriteString(fmt.Sprintf("| %s | %d | %s |\n", nameWithDesc, cs.ReposWithFail, strings.Join(cs.Representative, ", ")))
		}
		b.WriteString("\n")
	}

	// --- Top Riskiest Repos ---
	b.WriteString("## Top Riskiest Repos\n\n")
	riskiest := topRiskiestRepos(perRepo, 5)
	if len(riskiest) == 0 {
		b.WriteString("No risky repos found.\n\n")
	} else {
		b.WriteString("| Repo | FAIL | ERROR | Key Risks |\n")
		b.WriteString("| --- | ---: | ---: | --- |\n")
		for _, rs := range riskiest {
			risks := rs.KeyRisks()
			riskStr := strings.Join(risks, ", ")
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %s |\n", rs.Repo, rs.Fail, rs.Error, riskStr))
		}
		b.WriteString("\n")
	}

	// --- Monday Morning Hit List ---
	if len(riskiest) > 0 {
		b.WriteString("### Monday Morning Hit List\n\n")
		b.WriteString("| Repo | First Fix |\n")
		b.WriteString("| --- | --- |\n")
		for _, rs := range riskiest {
			// Determine First Fix
			firstFix := "Assign ownership + documentation baseline (CODEOWNERS/README/description)"

			hasFail := func(ruleID string) bool {
				for _, r := range rs.Results {
					if r.RuleID == ruleID && r.Status == rules.StatusFail {
						return true
					}
				}
				return false
			}

			hasCategoryFail := func(cat string) bool {
				for _, r := range rs.Results {
					if r.Status == rules.StatusFail && getCategory(r.RuleID) == cat {
						return true
					}
				}
				return false
			}

			if hasCategoryFail(CategoryDefaultBranchProtections) || hasFail("default-branch-protected") || hasFail("default-branch-required-status-checks") {
				firstFix = "Enforce default-branch gates (protection + checks + PR)"
			} else if hasFail("repo-visibility-public") {
				firstFix = "Confirm intentional public; otherwise restrict or allow-list"
			} else if hasFail("branch-protect-enforce-admins") || hasFail("protected-branches-block-deletion") {
				firstFix = "Ensure protections apply to administrators / block deletion"
			} else if hasFail("secret-scanning-disabled") {
				firstFix = "Enable secret scanning (if available)"
			}

			b.WriteString(fmt.Sprintf("| %s | %s |\n", rs.Repo, firstFix))
		}
		b.WriteString("\n")
	}

	// --- Minimum Baseline ---
	b.WriteString("### Minimum Baseline Checklist\n\n")
	b.WriteString("RepoMedic recommends establishing this baseline on all active repositories:\n\n")
	b.WriteString("- [ ] **Require Pull Requests**: Enable \"Require a pull request before merging\" on the default branch.\n")
	b.WriteString("- [ ] **Status Checks**: Enable \"Require status checks to pass before merging\" (at least one CI job).\n")
	b.WriteString("- [ ] **No Force Pushes**: Enable \"Allow force pushes\" = false (usually default for protected branches).\n")
	b.WriteString("- [ ] **Restrict Pushes**: Limit \"Who can push\" to specific teams or disable direct pushes entirely.\n")
	b.WriteString("- [ ] **Secret Scanning**: Enable GitHub Secret Scanning if available on your plan.\n\n")

	// --- Overall Risk Posture ---
	b.WriteString("## Overall Risk Posture\n\n")
	b.WriteString(fmt.Sprintf("RepoMedic scanned %d repositories. ", len(repos)))
	b.WriteString("See the Executive Risk Brief above for critical counts.\n\n")

	var priorities []string
	if unsafeDefaultBranch > 0 {
		priorities = append(priorities, "lock down default branches")
	}
	if publicFail > 0 {
		priorities = append(priorities, "restrict public access")
	}
	if audit.Blocked > 0 {
		priorities = append(priorities, "resolve audit blockers")
	}

	if len(priorities) > 0 {
		limit := 2
		if len(priorities) < limit {
			limit = len(priorities)
		}
		b.WriteString(fmt.Sprintf("Immediate priority should be to %s.\n\n", strings.Join(priorities[:limit], " and ")))
	} else {
		b.WriteString("Maintain current security posture.\n\n")
	}

	// --- Audit Coverage ---
	b.WriteString("## Audit Coverage\n\n")
	b.WriteString("RepoMedic does not guess when coverage is incomplete. Coverage blockers are reported as ERROR to avoid false PASS results.\n")
	b.WriteString("Blocked coverage creates governance blind spots and can hide drift. Resolving these blockers is critical to ensure accurate risk assessment.\n")
	b.WriteString("Note: While some remediation steps may depend on GitHub plan limits, 403 errors mean the settings are completely invisible to the scanner.\n\n")

	if len(audit.Blockers) == 0 {
		b.WriteString("No blockers found.\n\n")
	} else {
		b.WriteString("### Blockers\n")
		for _, bInfo := range audit.Blockers {
			// count + example repos (max 3 + “+N more”)
			repoList := formatRepoList(bInfo.ExampleRepos, 3)
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", bInfo.Reason, repoList))

			if len(bInfo.ImpactedRules) > 0 {
				// Map rule IDs to categories
				cats := make(map[string]struct{})
				for _, ruleID := range bInfo.ImpactedRules {
					cats[getCategory(ruleID)] = struct{}{}
				}
				var catList []string
				for c := range cats {
					catList = append(catList, c)
				}
				sort.Strings(catList)
				b.WriteString(fmt.Sprintf("  - Impacted areas: %s\n", strings.Join(catList, ", ")))
			}
		}
		b.WriteString("\n")
	}

	// --- Per-Repo Risk Table ---
	b.WriteString("## Per-repo status\n")
	b.WriteString("| Repo | FAIL | ERROR | Key Risks |\n")
	b.WriteString("| --- | ---: | ---: | --- |\n")

	// Sort repos by risk
	sortedRepos := make([]*repoStats, 0, len(perRepo))
	for _, rs := range perRepo {
		sortedRepos = append(sortedRepos, rs)
	}
	sort.Slice(sortedRepos, func(i, j int) bool {
		if sortedRepos[i].Fail != sortedRepos[j].Fail {
			return sortedRepos[i].Fail > sortedRepos[j].Fail
		}
		if sortedRepos[i].Error != sortedRepos[j].Error {
			return sortedRepos[i].Error > sortedRepos[j].Error
		}
		return sortedRepos[i].Repo < sortedRepos[j].Repo
	})

	for _, rs := range sortedRepos {
		risks := rs.KeyRisks()
		riskStr := strings.Join(risks, ", ")
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %s |\n", rs.Repo, rs.Fail, rs.Error, riskStr))
	}
	b.WriteString("\n")

	// --- Critical Findings ---
	b.WriteString("## Critical findings\n\n")
	if len(fails) == 0 {
		b.WriteString("- None\n\n")
	} else {
		// Group by repo, then sort by priority
		failsByRepo := make(map[string][]rules.Result)
		for _, f := range fails {
			failsByRepo[f.Repo] = append(failsByRepo[f.Repo], f)
		}

		for _, repo := range repos {
			rs, ok := failsByRepo[repo]
			if !ok {
				continue
			}
			// Sort findings by priority
			sort.Slice(rs, func(i, j int) bool {
				p1 := getPriority(rs[i].RuleID)
				p2 := getPriority(rs[j].RuleID)
				if p1 != p2 {
					return p1 < p2
				}
				return rs[i].RuleID < rs[j].RuleID
			})

			b.WriteString(fmt.Sprintf("### %s\n", repo))

			// Group default branch failures
			var defaultBranchFailures []rules.Result
			var otherFailures []rules.Result

			for _, r := range rs {
				cat := getCategory(r.RuleID)
				if cat == CategoryDefaultBranchProtections || cat == CategoryNoBranchProtection {
					defaultBranchFailures = append(defaultBranchFailures, r)
				} else {
					otherFailures = append(otherFailures, r)
				}
			}

			printResult := func(r rules.Result) {
				b.WriteString(fmt.Sprintf("- **%s**", r.RuleID))
				if r.Message != "" {
					b.WriteString(fmt.Sprintf(": %s", r.Message))
				}
				b.WriteString("\n")
				if len(r.Evidence) > 0 {
					keys := make([]string, 0, len(r.Evidence))
					for k := range r.Evidence {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("  - %s: %s\n", k, r.Evidence[k]))
					}
				}
			}

			if len(defaultBranchFailures) > 0 {
				b.WriteString("#### Default branch lacks enforced protections:\n")
				for _, r := range defaultBranchFailures {
					printResult(r)
				}
			}

			if len(otherFailures) > 0 {
				if len(defaultBranchFailures) > 0 {
					b.WriteString("#### Other findings:\n")
				}
				for _, r := range otherFailures {
					printResult(r)
				}
			}
			b.WriteString("\n")
		}
	}

	// --- Warnings ---
	b.WriteString("## Warnings\n\n")
	if len(skips) == 0 {
		b.WriteString("- None\n\n")
	} else {
		// Group by rule ID
		skipsByRule := make(map[string][]string) // ruleID -> repos
		for _, s := range skips {
			skipsByRule[s.RuleID] = append(skipsByRule[s.RuleID], s.Repo)
		}
		var skipRules []string
		for id := range skipsByRule {
			skipRules = append(skipRules, id)
		}
		sort.Strings(skipRules)

		for _, id := range skipRules {
			affected := skipsByRule[id]
			sort.Strings(affected)
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", id, formatRepoList(affected, 5)))
		}
		b.WriteString("\n")
	}

	// --- Errors ---
	b.WriteString("## Errors\n\n")
	if len(errs) == 0 {
		b.WriteString("- None\n\n")
	} else {
		// Group by rule ID
		errsByRule := make(map[string][]string) // ruleID -> repos
		for _, e := range errs {
			errsByRule[e.RuleID] = append(errsByRule[e.RuleID], e.Repo)
		}
		var errRules []string
		for id := range errsByRule {
			errRules = append(errRules, id)
		}
		sort.Strings(errRules)

		for _, id := range errRules {
			affected := errsByRule[id]
			sort.Strings(affected)
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", id, formatRepoList(affected, 5)))
		}
		b.WriteString("\n")
	}

	// --- Rules Evaluated ---
	b.WriteString("## Rules evaluated\n")
	var ruleIDs []string
	for id := range uniqueRules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	if len(ruleIDs) == 0 {
		b.WriteString("- None\n\n")
	} else {
		for _, id := range ruleIDs {
			b.WriteString(fmt.Sprintf("- %s\n", id))
		}
		b.WriteString("\n")
	}

	if _, err := s.file.WriteString(b.String()); err != nil {
		return writeErr(err)
	}
	return s.file.Close()
}
