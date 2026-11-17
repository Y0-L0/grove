package workflow

import (
	"errors"
	"fmt"

	"github.com/nicksenap/grove/internal/config"
	"github.com/nicksenap/grove/internal/contextrepo"
	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/lifecycle"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/operations"
	"github.com/nicksenap/grove/internal/state"
)

// Hooks implements the fork's workspace lifecycle behavior.
type Hooks struct{}

func (Hooks) ResolveBranch(req operations.CreateRequest) (string, error) {
	if req.Options.Branch != "" {
		return req.Options.Branch, nil
	}
	if req.Options.Cfg == nil || req.Options.Cfg.OwnerPrefix == "" {
		return "", errors.New("set owner_prefix in config.toml or supply --branch")
	}
	return req.Options.Cfg.OwnerPrefix + "/" + req.Name, nil
}

func (Hooks) PostCreate(ws models.Workspace) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := createRecords(ws, cfg); err != nil {
		return err
	}
	if cfg != nil && cfg.Types[ws.Type].Context != "" {
		// Records creation marks explicitly selected context repositories too.
		current, err := state.NewStore(cfg.StateDir).GetWorkspace(ws.Name)
		if err != nil {
			return fmt.Errorf("loading workspace after context creation: %w", err)
		}
		if current == nil {
			return fmt.Errorf("workspace %s disappeared after context creation", ws.Name)
		}
		ws = *current
	}
	return configureRemotes(ws)
}

func (Hooks) PostAddRepo(ws models.Workspace) error { return configureRemotes(ws) }

func configureRemotes(ws models.Workspace) error {
	for _, repo := range ws.Repos {
		if repo.Context {
			continue
		}
		if err := gitops.EnsureSSHPushURL(repo.SourceRepo); err != nil {
			return fmt.Errorf("configuring push URL for %s: %w", repo.RepoName, err)
		}
	}
	return nil
}

func (Hooks) PreDelete(ws models.Workspace) error { return contextrepo.CheckDelete(ws.Repos) }

func (Hooks) OnClose() error { return lifecycle.ErrNoHook }
