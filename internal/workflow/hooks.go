package workflow

import (
	"errors"

	"github.com/nicksenap/grove/internal/config"
	"github.com/nicksenap/grove/internal/contextrepo"
	"github.com/nicksenap/grove/internal/lifecycle"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/operations"
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
	return createRecords(ws, cfg)
}

func (Hooks) PostAddRepo(models.Workspace) error { return lifecycle.ErrNoHook }

func (Hooks) PreDelete(ws models.Workspace) error { return contextrepo.CheckDelete(ws.Repos) }

func (Hooks) OnClose() error { return lifecycle.ErrNoHook }
