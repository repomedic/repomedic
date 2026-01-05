package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "repomedic",
	Short: "Scan GitHub repositories and report objective governance divergences",
	Long: `RepoMedic scans GitHub repositories via API and reports objective governance divergences.

RepoMedic is scan-only: it finds wrongs, does not fix them, and does not moralize.

Examples:
	# Show available commands and global flags
	repomedic --help

	# Scan a repository
	repomedic scan --repos org/repo

	# List rules
	repomedic rules list

	# Print build info
	repomedic version

Output:
	By default, commands write human-readable output to stdout.
	Some commands support structured output via emitter flags (see each command's --help).`,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&cfg.Runtime.Verbose, "verbose", false, "Enable verbose logging (prints every GitHub API call and full error details)")
}

func SetBuildInfo(version, commit, date string) {
	if version != "" {
		buildVersion = version
	}
	if commit != "" {
		buildCommit = commit
	}
	if date != "" {
		buildDate = date
	}

	rootCmd.Version = fmt.Sprintf("%s (%s) %s", buildVersion, buildCommit, buildDate)
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func BuildInfo() (version, commit, date string) {
	return buildVersion, buildCommit, buildDate
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
