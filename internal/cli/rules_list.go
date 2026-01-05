package cli

import (
	"fmt"
	"io"

	"repomedic/internal/rules"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var rulesListQuiet bool
var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage and list rules",
	Long: `Manage RepoMedic rules.

This command group helps you discover which rules exist and what each rule checks.
Rules are evaluated during scans (see "repomedic scan --help").

Examples:
  # List all available rules
  repomedic rules list
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available rules",
	Long: `List all rules currently registered in this build.

Rules are sorted by rule ID.

Examples:
  repomedic rules list

Output:
  A vertical list of rules:
    ----------------------------------------
    RULE: {ID}
    ----------------------------------------
    {TITLE}
    {DESCRIPTION}
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rList := rules.List()

		for _, r := range rList {
			if rulesListQuiet {
				fmt.Fprintln(cmd.OutOrStdout(), r.ID())
			} else {
				printRule(cmd.OutOrStdout(), r)
			}
		}
		return nil
	},
}

var rulesShowCmd = &cobra.Command{
	Use:   "show [rule-id]",
	Short: "Show details of a specific rule",
	Long: `Show details of a specific rule by its ID.

Examples:
  repomedic rules show readme-root-exists
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rList, err := rules.Resolve(args[0])
		if err != nil {
			return err
		}
		if len(rList) == 0 {
			return fmt.Errorf("rule not found: %s", args[0])
		}
		printRule(cmd.OutOrStdout(), rList[0])
		return nil
	},
}

func printRule(w io.Writer, r rules.Rule) {
	bold := color.New(color.Bold)
	fmt.Fprintln(w, "----------------------------------------")
	bold.Fprintf(w, "RULE: %s\n", r.ID())
	fmt.Fprintln(w, "----------------------------------------")
	fmt.Fprintln(w, r.Title())
	fmt.Fprintln(w, r.Description())

	if cr, ok := r.(rules.ConfigurableRule); ok {
		opts := cr.Options()
		if len(opts) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Options:")
			for _, opt := range opts {
				def := opt.Default
				if def == "" {
					def = "\"\""
				}
				fmt.Fprintf(w, "  %s\n", opt.Name)
				fmt.Fprintf(w, "    Description: %s\n", opt.Description)
				fmt.Fprintf(w, "    Default:     %s\n", def)
			}
		}
	}
	fmt.Fprintln(w)
}

func init() {
	rootCmd.AddCommand(rulesCmd)
	rulesCmd.AddCommand(rulesListCmd)
	rulesListCmd.Flags().BoolVarP(&rulesListQuiet, "quiet", "q", false, "Only print rule IDs")
	rulesCmd.AddCommand(rulesShowCmd)
}
