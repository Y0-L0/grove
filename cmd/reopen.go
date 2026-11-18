package cmd

import (
	"fmt"
	"strings"

	"github.com/nicksenap/grove/internal/config"
	"github.com/nicksenap/grove/internal/workflow"
	"github.com/spf13/cobra"
)

var reopenCmd = &cobra.Command{
	Use:   "reopen NAME",
	Short: "Reopen deleted workspace records without code repositories",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repos, err := workflow.Reopen(args[0], config.RequireConfig())
		if err != nil {
			exitError(err.Error())
		}
		quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
		if len(repos) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "gw add-repo %s \\\n  --repos %s\n", quote(args[0]), quote(strings.Join(repos, ",")))
		}
	},
}
