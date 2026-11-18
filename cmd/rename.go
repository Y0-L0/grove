package cmd

import (
	"github.com/nicksenap/grove/internal/operations"
	"github.com/nicksenap/grove/internal/picker"
	"github.com/nicksenap/grove/internal/state"
	"github.com/spf13/cobra"
)

var renameTo string

var renameCmd = &cobra.Command{
	Use:   "rename [NAME] [NEW]",
	Short: "Rename a workspace",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			workspaces, err := state.Load()
			if err != nil {
				exitError(err.Error())
			}
			if len(workspaces) == 0 {
				exitError("No workspaces")
			}
			choices := make([]string, len(workspaces))
			for i, ws := range workspaces {
				choices[i] = ws.Name
			}
			selected, err := picker.PickOne("Select workspace to rename:", choices)
			if err != nil {
				exitOnPickerErr(err)
			}
			name = selected
		}

		newName := renameTo
		if len(args) == 2 {
			if newName != "" {
				exitError("supply NEW or --to, not both")
			}
			newName = args[1]
		}
		if newName == "" {
			exitError("--to is required")
		}

		if err := operations.NewService().Rename(name, newName); err != nil {
			exitError(err.Error())
		}
	},
}

func init() {
	renameCmd.Flags().StringVar(&renameTo, "to", "", "New workspace name")
	renameCmd.ValidArgsFunction = completeWorkspaceNames
}
