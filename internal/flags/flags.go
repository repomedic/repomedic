package flags

// Package flags defines canonical CLI flag names shared across the CLI and engine.
// Keeping these as constants helps avoid drift between Cobra flag wiring and other
// code paths that need to reference flags (e.g. report reproducibility command
// generation).
// IMPORTANT: These are flag *names* without leading dashes.
// Example usage:
//
//	cmd.Flags().StringVar(&cfg.Targeting.Org, flags.FlagOrg, "", "...")
//	arg := "--" + flags.FlagOrg
const (
	// Targeting
	FlagOrg        = "org"
	FlagUser       = "user"
	FlagEnterprise = "enterprise"
	FlagRepos      = "repos"
	FlagInclude    = "include"
	FlagExclude    = "exclude"
	FlagTopic      = "topic"
	FlagVisibility = "visibility"
	FlagArchived   = "archived"
	FlagForks      = "forks"
	FlagMaxRepos   = "max-repos"
	FlagDryRun     = "dry-run"

	// Rules
	FlagRules    = "rules"
	FlagSet      = "set"
	FlagEvidence = "evidence"

	// Output
	FlagConsoleFormat      = "console-format"
	FlagConsoleFilterStatus = "console-filter-status"
	FlagReport             = "report"
	FlagOut                = "out"
	FlagOutFormat          = "out-format"
	FlagEmit               = "emit"
	FlagNoConsole          = "no-console"

	// Runtime
	FlagConcurrency = "concurrency"
	FlagTimeout     = "timeout"
	FlagFailFast    = "fail-fast"
)
