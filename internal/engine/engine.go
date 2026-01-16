package engine

import (
	"context"
	"fmt"
	"os"
	"repomedic/internal/config"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"
	gh "repomedic/internal/github"
	"repomedic/internal/output"
	"repomedic/internal/rules"
	"sort"
	"strings"

	"github.com/google/go-github/v81/github"
)

func exitCodeForRun(fatal, partial, wrongs bool) int {
	// Exit code contract (UI spec):
	// 0 = clean run, no wrongs
	// 1 = wrongs detected
	// 2 = partial failure (some rules/repos errored)
	// 3 = fatal error (scan did not run)
	if fatal {
		return 3
	}
	if partial {
		return 2
	}
	if wrongs {
		return 1
	}
	return 0
}

func setupOutputManager(cfg *config.Config) (*output.Manager, error) {
	outMgr := output.NewManager()

	// Console Sink
	if !cfg.Output.NoConsole {
		if err := outMgr.AddSink(output.NewConsoleSink(nil, cfg.Output.ConsoleFormat, cfg.Output.ConsoleFilterStatus)); err != nil {
			outMgr.Close()
			return nil, err
		}
	}

	// Emit Sinks (additional structured streams)
	for _, emit := range cfg.Output.Emit {
		es, err := output.NewEmitSink(os.Stdout, emit)
		if err != nil {
			outMgr.Close()
			return nil, err
		}
		if err := outMgr.AddSink(es); err != nil {
			outMgr.Close()
			return nil, err
		}
	}

	// File Sink
	if cfg.Output.Out != "" {
		fs, err := output.NewFileSink(cfg.Output.Out, cfg.Output.OutFormat)
		if err != nil {
			outMgr.Close()
			return nil, err
		}
		if err := outMgr.AddSink(fs); err != nil {
			outMgr.Close()
			return nil, err
		}
	}

	// Report Sink
	if cfg.Output.Report != "" {
		rs, err := output.NewReportSink(cfg.Output.Report)
		if err != nil {
			outMgr.Close()
			return nil, err
		}
		if err := outMgr.AddSink(rs); err != nil {
			outMgr.Close()
			return nil, err
		}
	}

	return outMgr, nil
}

func applyRuleOptionsIfAny(cfg *config.Config) error {
	// applyRuleOptionsIfAny applies per-rule configuration supplied via repeated
	// --set flags.
	//
	// --set values are parsed as "ruleID.option=value" and routed to the matching
	// rule's Configure method (only rules that implement rules.ConfigurableRule).
	//
	// Example:
	//   repomedic scan --org my-org --set my-rule.my_option=true

	if len(cfg.Rules.Set) == 0 {
		return nil
	}

	assignments, err := config.ParseRuleOptionAssignments(cfg.Rules.Set)
	if err != nil {
		return err
	}

	all := rules.List()
	byID := make(map[string]rules.Rule, len(all))
	for _, r := range all {
		byID[r.ID()] = r
	}

	for ruleID, opts := range assignments {
		r, ok := byID[ruleID]
		if !ok {
			return fmt.Errorf("unknown rule ID %q", ruleID)
		}
		cr, ok := r.(rules.ConfigurableRule)
		if !ok {
			return fmt.Errorf("rule %q does not support options", ruleID)
		}

		allowed := make(map[string]struct{})
		for _, opt := range cr.Options() {
			allowed[opt.Name] = struct{}{}
		}
		for name := range opts {
			if _, ok := allowed[name]; !ok {
				return fmt.Errorf("unknown option %q for rule %q", name, ruleID)
			}
		}

		if err := cr.Configure(opts); err != nil {
			return fmt.Errorf("configure rule %q: %w", ruleID, err)
		}
	}

	return nil
}

// ruleResultIfDependenciesMissingOrFailed returns a synthetic rule status/message when required dependencies are missing or failed.
//
// In RepoMedic, a "dependency" is a required piece of GitHub-derived data identified by a data.DependencyKey.
// Those dependencies are fetched ahead of time and placed into the repo's data.DataContext; if a required
// key is missing from the DataContext (or failed to fetch), the rule can't be evaluated normally.
func ruleResultIfDependenciesMissingOrFailed(dc data.DataContext, deps []data.DependencyKey, repoDepErrs map[data.DependencyKey]error, verbose bool) (rules.Status, string, bool) {
	var missing []string
	var failedDepMessages []string
	hasSkippableFailure := false
	hasHardFailure := false

	for _, d := range deps {
		if _, ok := dc.Get(d); ok {
			continue
		}
		if repoDepErrs != nil {
			if depErr := repoDepErrs[d]; depErr != nil {
				pres := presentDependencyError(d, depErr, verbose)
				// If multiple deps fail, include the dependency key so the user can tell what failed.
				// If exactly one dep fails, emit only the message for a cleaner UX.
				failedDepMessages = append(failedDepMessages, fmt.Sprintf("%s: %s", d, pres.message))
				if pres.disposition == depErrDispositionSkip {
					hasSkippableFailure = true
				} else {
					hasHardFailure = true
				}
				continue
			}
		}
		missing = append(missing, string(d))
	}

	if len(failedDepMessages) > 0 {
		status := rules.StatusError
		if hasSkippableFailure && !hasHardFailure {
			status = rules.StatusSkipped
		}

		msg := strings.Join(failedDepMessages, "; ")
		if len(failedDepMessages) == 1 {
			if _, after, ok := strings.Cut(failedDepMessages[0], ": "); ok {
				msg = after
			}
		}
		return status, msg, true
	}

	if len(missing) > 0 {
		return rules.StatusError, fmt.Sprintf("Missing dependencies: %v", missing), true
	}

	return "", "", false
}

type Engine struct {
	Client *gh.Client

	// schedulerExecute is a test seam for streaming execution.
	// If nil, Engine uses the real fetcher + scheduler.
	schedulerExecute func(ctx context.Context, cfg *config.Config, plan *ScanPlan) (<-chan RepoExecutionResult, <-chan error)
}

func NewEngine(client *gh.Client) *Engine {
	return &Engine{
		Client: client,
	}
}

func (e *Engine) executePlanStream(ctx context.Context, cfg *config.Config, plan *ScanPlan) (<-chan RepoExecutionResult, <-chan error) {
	if e.schedulerExecute != nil {
		return e.schedulerExecute(ctx, cfg, plan)
	}

	// Initialize Fetcher
	// TODO: Get rate limit from client or config? For now use default.
	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(e.Client, budget)

	// Inject scanned repos list into fetcher so org-scoped dependencies
	// (like DepReposScanned) can access it without additional API calls.
	f.SetScannedRepos(extractReposFromPlan(plan))

	// Initialize Scheduler
	scheduler, err := NewScheduler(f, cfg.Runtime.Concurrency)
	if err != nil {
		resCh := make(chan RepoExecutionResult)
		errCh := make(chan error, 1)
		close(resCh)
		errCh <- err
		close(errCh)
		return resCh, errCh
	}
	return scheduler.Execute(ctx, plan)
}

// evaluateStreamingResults receives streamed per-repo execution results (fetched dependencies + any fetch errors),
// validates that each rule's required dependencies are present, executes rule logic, and forwards results/events to
// the configured output sinks.
func evaluateStreamingResults(ctx context.Context, cfg *config.Config, plan *ScanPlan, resCh <-chan RepoExecutionResult, outMgr *output.Manager) (hasErrors bool, hasFailures bool) {
	for res := range resCh {
		rp := plan.RepoPlans[res.RepoID]
		if rp == nil {
			hasErrors = true
			continue
		}

		repoFullName := fmt.Sprintf("%s/%s", rp.Repo.Owner, rp.Repo.Name)
		_ = outMgr.Write(output.Event{Type: "repo.started", Repo: repoFullName})

		dc := res.Data
		if dc == nil {
			dc = data.NewMapDataContext(map[data.DependencyKey]any{})
		}

		for _, rule := range rp.Rules {
			deps, err := rule.Dependencies(ctx, rp.Repo.Repo)
			if err != nil {
				_ = outMgr.Write(rules.Result{
					Repo:    repoFullName,
					RuleID:  rule.ID(),
					Status:  rules.StatusError,
					Message: fmt.Sprintf("Failed to determine dependencies: %v", err),
				})
				hasErrors = true
				continue
			}

			if status, msg, ok := ruleResultIfDependenciesMissingOrFailed(dc, deps, res.DepErrs, cfg.Runtime.Verbose); ok {
				_ = outMgr.Write(rules.Result{
					Repo:    repoFullName,
					RuleID:  rule.ID(),
					Status:  status,
					Message: msg,
				})
				if status == rules.StatusError {
					hasErrors = true
				}
				if status == rules.StatusFail {
					hasFailures = true
				}
				continue
			}

			// Enforce the rules contract: a rule must not read dependency keys it did
			// not declare in Dependencies(). This prevents rules from implicitly relying
			// on other rules' dependencies.
			tracked := data.NewTrackingDataContext(dc)
			ruleRes, err := rule.Evaluate(ctx, rp.Repo.Repo, tracked)
			undeclared := undeclaredDependencyAccesses(tracked.AccessedKeys(), deps)
			if len(undeclared) > 0 {
				msg := fmt.Sprintf("Rule accessed undeclared dependencies: %s. Declare them in Dependencies().", strings.Join(undeclared, ", "))
				if err != nil {
					msg = fmt.Sprintf("%s (evaluation error: %v)", msg, err)
				}
				_ = outMgr.Write(rules.Result{Repo: repoFullName, RuleID: rule.ID(), Status: rules.StatusError, Message: msg})
				hasErrors = true
				continue
			}
			if err != nil {
				_ = outMgr.Write(rules.Result{
					Repo:    repoFullName,
					RuleID:  rule.ID(),
					Status:  rules.StatusError,
					Message: fmt.Sprintf("Evaluation failed: %v", err),
				})
				hasErrors = true
				continue
			}

			// Backfill identifiers so output stays consistent and well-formed.
			// Rules usually care about PASS/FAIL + message/evidence; the engine already knows the repo and rule ID,
			// so we stamp them here to avoid repeated boilerplate and to keep sinks (ndjson/report/etc) happy.
			if ruleRes.Repo == "" {
				ruleRes.Repo = repoFullName
			}
			if ruleRes.RuleID == "" {
				ruleRes.RuleID = rule.ID()
			}

			switch ruleRes.Status {
			case rules.StatusFail:
				hasFailures = true
			case rules.StatusError:
				hasErrors = true
			}

			_ = outMgr.Write(ruleRes)
		}

		_ = outMgr.Write(output.Event{Type: "repo.finished", Repo: repoFullName})
	}

	return hasErrors, hasFailures
}

func undeclaredDependencyAccesses(accessed []data.DependencyKey, declared []data.DependencyKey) []string {
	if len(accessed) == 0 {
		return nil
	}
	decl := make(map[data.DependencyKey]struct{}, len(declared))
	for _, d := range declared {
		decl[d] = struct{}{}
	}

	var out []string
	for _, k := range accessed {
		if _, ok := decl[k]; ok {
			continue
		}
		out = append(out, string(k))
	}
	sort.Strings(out)
	return out
}

// extractReposFromPlan extracts the list of *github.Repository from a ScanPlan.
// This is used to inject the scanned repos into the fetcher for org-scoped dependencies.
func extractReposFromPlan(plan *ScanPlan) []*github.Repository {
	if plan == nil || plan.RepoPlans == nil {
		return nil
	}
	repos := make([]*github.Repository, 0, len(plan.RepoPlans))
	for _, rp := range plan.RepoPlans {
		if rp.Repo.Repo != nil {
			repos = append(repos, rp.Repo.Repo)
		}
	}
	return repos
}

func isExplicitReposOnly(cfg *config.Config) bool {
	return cfg.Targeting.Org == "" && cfg.Targeting.Enterprise == "" && len(cfg.Targeting.Repos) > 0
}

func (e *Engine) discoverRepos(ctx context.Context, cfg *config.Config, explicitReposOnly bool) ([]RepositoryRef, bool) {
	if !cfg.Output.NoConsole {
		if explicitReposOnly {
			fmt.Fprintln(os.Stderr, "Resolving repositories...")
		} else {
			fmt.Fprintln(os.Stderr, "Discovering repositories...")
		}
	}
	repos, err := ResolveRepos(ctx, e.Client, cfg)
	if err != nil {
		if explicitReposOnly {
			fmt.Fprintf(os.Stderr, "Error resolving repositories: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Error discovering repositories: %v\n", err)
		}
		return nil, false
	}
	return repos, true
}

func filterReposIfNeeded(repos []RepositoryRef, cfg *config.Config, explicitReposOnly bool) []RepositoryRef {
	// If the user explicitly provided repos (and did not use org/enterprise discovery),
	// treat the repo list as exact: do not filter out explicitly targeted repos.
	if explicitReposOnly {
		return repos
	}
	return FilterRepos(repos, cfg)
}

func maybeDryRun(cfg *config.Config, repos []RepositoryRef) (int, bool) {
	if !cfg.Targeting.DryRun {
		return 0, false
	}

	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, fmt.Sprintf("%s/%s", r.Owner, r.Name))
	}
	sort.Strings(names)
	fmt.Println("Resolved repositories:")
	for _, n := range names {
		fmt.Println(n)
	}
	return 0, true
}

func resolveAndConfigureRules(cfg *config.Config) ([]rules.Rule, bool) {
	if !cfg.Output.NoConsole {
		fmt.Fprintln(os.Stderr, "Resolving rules...")
	}
	selectedRules, err := rules.Resolve(cfg.Rules.Selector)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving rules: %v\n", err)
		return nil, false
	}

	if err := applyRuleOptionsIfAny(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error configuring rules: %v\n", err)
		return nil, false
	}

	if !cfg.Output.NoConsole {
		fmt.Fprintf(os.Stderr, "Selected %d rules.\n", len(selectedRules))
	}
	return selectedRules, true
}

func buildPlanForRepos(ctx context.Context, cfg *config.Config, repos []RepositoryRef, selectedRules []rules.Rule) (*ScanPlan, bool) {
	if !cfg.Output.NoConsole {
		fmt.Fprintln(os.Stderr, "Planning scan...")
	}
	plan := NewScanPlan()
	for _, repo := range repos {
		if err := plan.AddRepo(ctx, repo, selectedRules); err != nil {
			fmt.Fprintf(os.Stderr, "Error adding repo %s to plan: %v\n", repo.Name, err)
			return nil, false
		}
	}
	return plan, true
}

func (e *Engine) Run(ctx context.Context, cfg *config.Config) int {
	explicitReposOnly := isExplicitReposOnly(cfg)

	repos, ok := e.discoverRepos(ctx, cfg, explicitReposOnly)
	if !ok {
		return exitCodeForRun(true, false, false)
	}

	repos = filterReposIfNeeded(repos, cfg, explicitReposOnly)
	if !cfg.Output.NoConsole {
		fmt.Fprintf(os.Stderr, "Found %d repositories.\n", len(repos))
	}

	if code, ok := maybeDryRun(cfg, repos); ok {
		return code
	}

	selectedRules, ok := resolveAndConfigureRules(cfg)
	if !ok {
		return exitCodeForRun(true, false, false)
	}

	plan, ok := buildPlanForRepos(ctx, cfg, repos, selectedRules)
	if !ok {
		return exitCodeForRun(true, false, false)
	}

	outMgr, err := setupOutputManager(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output sinks: %v\n", err)
		return exitCodeForRun(true, false, false)
	}
	defer outMgr.Close()

	_ = outMgr.Write(output.Event{Type: "run.started", Repos: len(plan.RepoPlans), Rules: len(selectedRules)})

	resCh, errCh := e.executePlanStream(ctx, cfg, plan)

	hasErrors, hasFailures := evaluateStreamingResults(ctx, cfg, plan, resCh, outMgr)

	var schedErr error
	// Drain scheduler errors; we only need to know whether any fatal error occurred (keep one non-nil error).
	for err := range errCh {
		if err != nil {
			schedErr = err
		}
	}

	fatal := schedErr != nil
	code := exitCodeForRun(fatal, hasErrors, hasFailures)
	_ = outMgr.Write(output.Event{Type: "run.finished", ExitCode: code})
	return code
}
