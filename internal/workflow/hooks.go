package workflow

import (
	"errors"

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

func (Hooks) PostCreate(models.Workspace) error { return lifecycle.ErrNoHook }

func (Hooks) PostAddRepo(models.Workspace) error { return lifecycle.ErrNoHook }

func (Hooks) PreDelete(models.Workspace) error { return lifecycle.ErrNoHook }

func (Hooks) OnClose() error { return lifecycle.ErrNoHook }
