package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		version, commit, date := BuildInfo()
		fmt.Fprintf(cmd.OutOrStdout(), "repomedic %s\ncommit: %s\nbuilt:  %s\n", version, commit, date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
