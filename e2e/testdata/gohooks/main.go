package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nicksenap/grove/cmd"
	"github.com/nicksenap/grove/internal/lifecycle"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/operations"
)

func main() {
	operations.RegisterHooks(goHooks{}, operations.ShellHooks{})
	cmd.Execute()
}

type goHooks struct{}

func (goHooks) ResolveBranch(req operations.CreateRequest) (string, error) {
	return "go/" + req.Options.Branch, nil
}

func (goHooks) PostCreate(ws models.Workspace) error  { return record("post_create", ws) }
func (goHooks) PostAddRepo(ws models.Workspace) error { return record("post_add_repo", ws) }
func (goHooks) PreDelete(ws models.Workspace) error   { return record("pre_delete", ws) }
func (goHooks) OnClose() error                        { return lifecycle.ErrNoHook }

func record(phase string, ws models.Workspace) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var repos []string
	for _, repo := range ws.Repos {
		repos = append(repos, repo.RepoName)
	}
	f, err := os.OpenFile(filepath.Join(home, "hook-events"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString("go:" + phase + " " + ws.Name + " " + ws.Branch + " " + strings.Join(repos, ",") + "\n")
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
