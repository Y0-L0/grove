package cmd

import (
	"github.com/nicksenap/grove/internal/workflow"
	"github.com/spf13/cobra"
)

var signCmd = &cobra.Command{
	Use:   "sign [NAME]",
	Short: "Show, sign and push the workspace code branches",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		ws := lookupWorkspaceForShow(name)
		if err := workflow.Sign(*ws, cmd.OutOrStdout()); err != nil {
			exitError(err.Error())
		}
	},
	ValidArgsFunction: completeWorkspaceNames,
}
