package operations

import (
	"errors"
	"fmt"

	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/lifecycle"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/workspace"
)

// Hooks receives typed context at each workspace lifecycle phase.
// ResolveBranch returns the branch for the next implementation. Returning
// lifecycle.ErrNoHook skips an implementation. Resolver and OnClose errors abort;
// other phases also accept non-aborting lifecycle.HookError values as warnings.
type Hooks interface {
	ResolveBranch(CreateRequest) (string, error)
	PostCreate(models.Workspace) error
	PostAddRepo(models.Workspace) error
	PreDelete(models.Workspace) error
	OnClose() error
}

// DeletePolicy runs even with --no-hooks, before optional pre_delete hooks.
// It must preserve committed context records before authorizing removal.
type DeletePolicy interface {
	PrepareDelete(models.Workspace, workspace.RemoveOptions) error
}

// CreatePolicy checks required constraints even with optional hooks disabled.
type CreatePolicy interface {
	CheckCreate(CreateRequest) error
}

// RenamePolicy preserves records when a workspace name changes.
// It applies even when optional lifecycle hooks are disabled.
type RenamePolicy interface {
	CheckRename(models.Workspace, string) error
	RenameRecords(models.Workspace, string) error
}

var registeredHooks = []Hooks{ShellHooks{}}

// RegisterHooks sets the lifecycle implementations in execution order.
// Call it once at startup, before running commands or creating services.
// Register Go implementations before ShellHooks to prepare their shell context.
func RegisterHooks(hooks ...Hooks) {
	registeredHooks = append([]Hooks{}, hooks...)
}

// ShellHooks runs the lifecycle commands configured in config.toml.
type ShellHooks struct{}

func (ShellHooks) ResolveBranch(req CreateRequest) (string, error) {
	return lifecycle.Resolve("resolve_branch", lifecycleVars(models.Workspace{
		Name: req.Name, Branch: req.Options.Branch, Source: req.Options.Source,
	}))
}

func (ShellHooks) PostCreate(ws models.Workspace) error {
	return lifecycle.Run("post_create", lifecycleVars(ws))
}

func (ShellHooks) PostAddRepo(ws models.Workspace) error {
	return lifecycle.Run("post_add_repo", lifecycleVars(ws))
}

func (ShellHooks) PreDelete(ws models.Workspace) error {
	return lifecycle.Run("pre_delete", lifecycleVars(ws))
}

func (ShellHooks) OnClose() error {
	return lifecycle.Run("on_close", lifecycle.Vars{})
}

func (s *Service) hooks() []Hooks {
	if s.Hooks != nil {
		return s.Hooks
	}
	return registeredHooks
}

func (s *Service) runHooks(call func(Hooks) error) error {
	if lifecycle.Disabled {
		return nil
	}
	for _, h := range s.hooks() {
		err := call(h)
		if err == nil || errors.Is(err, lifecycle.ErrNoHook) {
			continue
		}
		var hookErr *lifecycle.HookError
		if !errors.As(err, &hookErr) || lifecycle.ShouldAbort(err) {
			return err
		}
		s.warn(err)
	}
	return nil
}

// callResolver returns the branch produced by the resolve_branch hook, or the
// typed branch unchanged when no resolver is configured. With hooks disabled,
// an omitted branch defaults to the workspace name.
// {path} is empty: the workspace does not exist yet.
func (s *Service) callResolver(req CreateRequest) (string, error) {
	if lifecycle.Disabled {
		if req.Options.Branch == "" {
			return req.Name, nil
		}
		return req.Options.Branch, nil
	}
	for _, hooks := range s.hooks() {
		branch, err := hooks.ResolveBranch(req)
		if errors.Is(err, lifecycle.ErrNoHook) {
			continue
		}
		if err != nil {
			return "", err
		}
		if err := gitops.CheckBranchName(branch); err != nil {
			return "", fmt.Errorf("resolve_branch hook output: %w", err)
		}
		req.Options.Branch = branch
	}
	return req.Options.Branch, nil
}

func lifecycleVars(ws models.Workspace) lifecycle.Vars {
	vars := lifecycle.Vars{Name: ws.Name, Path: ws.Path, Branch: ws.Branch}
	if ws.Source != nil {
		vars.SourceURL = ws.Source.URL
		vars.SourceRef = ws.Source.Ref
		vars.SourceTitle = ws.Source.Title
	}
	return vars
}
