package main

import (
	"github.com/nicksenap/grove/cmd"
	"github.com/nicksenap/grove/internal/operations"
	"github.com/nicksenap/grove/internal/workflow"
)

func main() {
	operations.RegisterHooks(workflow.Hooks{}, operations.ShellHooks{})
	cmd.Execute()
}
