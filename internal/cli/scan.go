package cli

import (
	"context"
	"fmt"
	"os"
	"repomedic/internal/config"
	"repomedic/internal/engine"
	"repomedic/internal/flags"
	gh "repomedic/internal/github"
	"strings"

	"github.com/spf13/cobra"
)

var cfg = config.New()

const scanHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}Usage:
  {{.UseLine}}

{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}{{if .HasAvailableInheritedFlags}}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}Environment:
	RepoMedic authenticates to GitHub using an access token.

	Sources (in order):
	1) GITHUB_TOKEN environment variable
	2) GitHub CLI (gh) authentication via gh auth token (if gh is installed and logged in)

  Token guidance (brief):
  - PAT (classic): typically needs repo (to read private repos) and read:org
    (to enumerate org repositories).
  - Fine-grained PAT: grant access to the target repositories with
    Metadata: Read and Administration: Read.

  Examples:
    # macOS/Linux
    export GITHUB_TOKEN="<your_token>"
    repomedic scan --org my-org

		# GitHub CLI auth
		gh auth login
		repomedic scan --org my-org

    # Windows PowerShell
    $env:GITHUB_TOKEN = "<your_token>"
    repomedic scan --org my-org

{{if .HasAvailableSubCommands}}Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

{{end}}{{if .HasHelpSubCommands}}Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

{{end}}{{if .HasAvailableSubCommands}}Use "{{.CommandPath}} [command] --help" for more information about a command.
{{end}}`

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a set of GitHub repositories",
	Long: `Scan a set of GitHub repositories and report objective wrongs.

RepoMedic is scan-only: it reads repository metadata and settings via the
GitHub API and never mutates state.

Authentication:
  RepoMedic uses a GitHub access token. It prefers GITHUB_TOKEN, but can also
  reuse GitHub CLI authentication if the gh CLI is installed and logged in.

Output:
	Console output is controlled by --console-format (default: text).
	Structured outputs can be written via:
	- --out / --out-format: write an aggregate JSON array or NDJSON stream to a file
	- --emit: write an additional structured stream to stdout (json or ndjson)
	- --no-console: suppress the console sink (use with --emit/--out for machine output)

	NDJSON mode emits one JSON object per line. Objects are lifecycle Events with a
	"type" field (run.started, repo.started, rule.result, repo.finished, run.finished).
	Rule results are represented as an Event with type "rule.result" and a nested
	"result" object.

Exit codes:
	0 = clean run, no wrongs
	1 = wrongs detected
	2 = partial failure (some rules/repos errored)
	3 = fatal error (scan did not run)

Examples:
  # Token via environment variable
  export GITHUB_TOKEN="<your_token>"
  repomedic scan --org my-org

  # Token via GitHub CLI auth
  gh auth login
	repomedic scan --user https://github.com/octocat

	# AI Agent: stream machine-readable events to stdout
	repomedic scan --org my-org --no-console --emit ndjson
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			_ = cmd.Help()
			return
		}

		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(3)
		}

		applyImplicitDefaults(cmd, cfg)

		ctx := context.Background()
		token, _, err := gh.ResolveAuthToken(ctx, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to resolve GitHub auth token: %v\n", err)
			os.Exit(3)
		}
		if strings.TrimSpace(token) == "" {
			fmt.Fprintln(os.Stderr, "Error: GitHub auth token is required (set GITHUB_TOKEN or run 'gh auth login')")
			os.Exit(3)
		}

		client, err := gh.NewClient(ctx, token, gh.WithVerbose(cfg.Runtime.Verbose, nil))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create GitHub client: %v\n", err)
			os.Exit(3)
		}
		eng := engine.NewEngine(client)
		os.Exit(eng.Run(ctx, cfg))
	},
}

func applyImplicitDefaults(cmd *cobra.Command, cfg *config.Config) {
	// When scanning a user account, include forks by default. Many GitHub users
	// have a significant portion of their repos as forks, and excluding them by
	// default is surprising.
	if cfg.Targeting.User != "" && cmd != nil {
		if !cmd.Flags().Changed(flags.FlagForks) {
			cfg.Targeting.Forks = "include"
		}
	}
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.SetHelpTemplate(scanHelpTemplate)

	// MAINTAINER NOTE: If you add/change/remove any scan-affecting flags here,
	// keep the report reproducibility command generator in sync:
	// internal/engine/engine.go:buildReproducibilityCommand.
	//
	// Output flags are intentionally omitted from the reproducibility command.

	// Targeting
	scanCmd.Flags().StringVar(&cfg.Targeting.Org, flags.FlagOrg, "", "GitHub organization account to scan (name or URL)")
	scanCmd.Flags().StringVar(&cfg.Targeting.User, flags.FlagUser, "", "GitHub user account to scan (name or URL)")
	scanCmd.Flags().StringVar(&cfg.Targeting.Enterprise, flags.FlagEnterprise, "", "GitHub enterprise to scan (not yet implemented)")
	scanCmd.Flags().StringSliceVar(&cfg.Targeting.Repos, flags.FlagRepos, nil, "Repositories to scan as OWNER/REPO (repeatable; comma-separated accepted)")
	scanCmd.Flags().StringSliceVar(&cfg.Targeting.Include, flags.FlagInclude, nil, "Include pattern(s) (repeatable; comma-separated accepted). Go path.Match style; if pattern contains '/', matches OWNER/REPO, else matches repo name")
	scanCmd.Flags().StringSliceVar(&cfg.Targeting.Exclude, flags.FlagExclude, nil, "Exclude pattern(s) (repeatable; comma-separated accepted). Same matching rules as --include")
	scanCmd.Flags().StringSliceVar(&cfg.Targeting.Topic, flags.FlagTopic, nil, "Require at least one topic match (repeatable; comma-separated accepted; exact match)")
	scanCmd.Flags().StringVar(&cfg.Targeting.Visibility, flags.FlagVisibility, "all", "Visibility filter: public|private|internal|all (default: all)")
	scanCmd.Flags().StringVar(&cfg.Targeting.Archived, flags.FlagArchived, "exclude", "Archived repos policy: include|exclude|only (default: exclude)")
	scanCmd.Flags().StringVar(&cfg.Targeting.Forks, flags.FlagForks, "exclude", "Forks policy: include|exclude|only (default: exclude). If --user is set and this flag is omitted, forks default to include")
	scanCmd.Flags().IntVar(&cfg.Targeting.MaxRepos, flags.FlagMaxRepos, 0, "Maximum number of repositories to scan (0 = unlimited)")
	scanCmd.Flags().BoolVar(&cfg.Targeting.DryRun, flags.FlagDryRun, false, "Resolve repos and print plan without scanning (still requires auth token)")

	// Rules
	scanCmd.Flags().StringVar(&cfg.Rules.Selector, flags.FlagRules, "", "Rule selector expression (empty = all rules)")
	scanCmd.Flags().StringSliceVar(&cfg.Rules.Set, flags.FlagSet, nil, "Per-rule options as ruleID.option=value (repeatable; comma-separated accepted)")
	scanCmd.Flags().StringVar(&cfg.Rules.Evidence, flags.FlagEvidence, "standard", "Evidence verbosity: minimal|standard|full (default: standard)")

	// Output
	scanCmd.Flags().StringVar(&cfg.Output.ConsoleFormat, flags.FlagConsoleFormat, "text", "Console output format: text|json|ndjson (default: text)")
	scanCmd.Flags().StringSliceVar(&cfg.Output.ConsoleFilterStatus, flags.FlagConsoleFilterStatus, nil, "Filter console output by status (PASS, FAIL, ERROR). Comma-separated.")
	scanCmd.Flags().StringVar(&cfg.Output.Report, flags.FlagReport, "", "Write a Markdown report to this path")
	scanCmd.Flags().StringVar(&cfg.Output.Out, flags.FlagOut, "", "Write structured output to this path")
	scanCmd.Flags().StringVar(&cfg.Output.OutFormat, flags.FlagOutFormat, "", "Structured output format for --out: json|ndjson (default: inferred from file extension)")
	scanCmd.Flags().StringSliceVar(&cfg.Output.Emit, flags.FlagEmit, nil, "Emit additional structured stream to stdout: json|ndjson (repeatable; comma-separated accepted)")
	scanCmd.Flags().BoolVar(&cfg.Output.NoConsole, flags.FlagNoConsole, false, "Suppress console output (use with --emit/--out/--report)")

	// Runtime
	scanCmd.Flags().IntVar(&cfg.Runtime.Concurrency, flags.FlagConcurrency, 5, "Concurrent workers (default: 5)")
	scanCmd.Flags().DurationVar(&cfg.Runtime.Timeout, flags.FlagTimeout, cfg.Runtime.Timeout, "Global timeout (default: 30m)")
	scanCmd.Flags().BoolVar(&cfg.Runtime.FailFast, flags.FlagFailFast, false, "Stop on first fatal error (default: false)")
}
