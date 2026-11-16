package main

import (
	"github.com/nicksenap/grove/cmd"
	"github.com/nicksenap/grove/internal/operations"
)

func main() {
	operations.RegisterHooks(operations.ShellHooks{})
	cmd.Execute()
}
