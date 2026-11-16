package operations

import (
	"errors"
	"testing"

	"github.com/nicksenap/grove/internal/lifecycle"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/workspace"
)

func TestCloseUsesGoHook(t *testing.T) {
	svc := Service{Hooks: []Hooks{testHooks{onClose: func() error { return nil }}, testHooks{}}}
	result, err := svc.Close(CloseRequest{})
	if err != nil || !result.Closed {
		t.Fatalf("close = %+v, %v, want success", result, err)
	}
}

func TestGoHookFailureStopsLaterHooks(t *testing.T) {
	blocked := errors.New("blocked by Go hook")
	for _, phase := range []string{"resolve_branch", "post_create", "post_add_repo", "pre_delete", "on_close"} {
		t.Run(phase, func(t *testing.T) {
			var order []string
			deps := &fakeWorkspace{workspace: models.Workspace{Name: "login", Branch: "owner/login"}, order: &order, added: 1}
			fail := func(models.Workspace) error { return blocked }
			unexpected := func(models.Workspace) error { t.Fatal("hook after failure ran"); return nil }
			first := testHooks{postCreate: fail, postAddRepo: fail, preDelete: fail,
				onClose: func() error { return blocked }}
			second := testHooks{postCreate: unexpected, postAddRepo: unexpected, preDelete: unexpected,
				onClose: func() error { t.Fatal("hook after failure ran"); return nil }}
			if phase == "resolve_branch" {
				first.resolve = func(CreateRequest) (string, error) { return "", blocked }
				second.resolve = func(CreateRequest) (string, error) { t.Fatal("resolver after failure ran"); return "", nil }
			}
			svc := Service{Workspace: deps, Store: deps, Hooks: []Hooks{first, second}}
			var err error
			switch phase {
			case "resolve_branch", "post_create":
				var result CreateResult
				result, err = svc.Create(CreateRequest{Name: "login", Options: workspace.CreateOpts{Branch: "owner/login"}})
				if result.Created != (phase == "post_create") {
					t.Fatalf("created = %v", result.Created)
				}
			case "post_add_repo":
				err = svc.AddRepos(AddReposRequest{Name: "login", Repos: []string{"svc-api"}})
			case "pre_delete":
				result, deleteErr := svc.Delete(DeleteRequest{Name: "login"})
				err = deleteErr
				if result.Deleted || len(order) != 0 {
					t.Fatalf("workspace deleted despite hook failure: %+v %v", result, order)
				}
			case "on_close":
				_, err = svc.Close(CloseRequest{})
			}
			var hookErr *HookError
			if !errors.Is(err, blocked) || !errors.As(err, &hookErr) || hookErr.Hook != phase {
				t.Fatalf("error = %v, want %s hook failure wrapping %v", err, phase, blocked)
			}
		})
	}
}

func TestGoResolversCompose(t *testing.T) {
	first := testHooks{resolve: func(req CreateRequest) (string, error) {
		return "owner/" + req.Options.Branch, nil
	}}
	second := testHooks{resolve: func(req CreateRequest) (string, error) {
		if req.Name != "login" || req.Options.Branch != "owner/login" {
			t.Fatalf("resolver request = %+v", req)
		}
		return req.Options.Branch + "-review", nil
	}}
	svc := Service{Hooks: []Hooks{first, testHooks{}, second}}
	req, err := svc.ResolveBranch(CreateRequest{Name: "login", Options: workspace.CreateOpts{Branch: "login"}})
	if err != nil {
		t.Fatal(err)
	}
	if req.Options.Branch != "owner/login-review" {
		t.Fatalf("branch = %q, want owner/login-review", req.Options.Branch)
	}
}

func TestInvalidGoBranchStopsResolution(t *testing.T) {
	svc := Service{Hooks: []Hooks{
		testHooks{resolve: func(CreateRequest) (string, error) { return "bad..branch", nil }},
		testHooks{resolve: func(CreateRequest) (string, error) { t.Fatal("resolver after invalid branch ran"); return "valid", nil }},
	}}
	_, err := svc.ResolveBranch(CreateRequest{Name: "login", Options: workspace.CreateOpts{Branch: "login"}})
	var hookErr *HookError
	if !errors.As(err, &hookErr) || hookErr.Hook != "resolve_branch" {
		t.Fatalf("error = %v, want resolve_branch failure", err)
	}
}

func TestWarningAllowsLaterGoHookAndDelete(t *testing.T) {
	var order []string
	var warnings []string
	deps := &fakeWorkspace{workspace: models.Workspace{Name: "login"}, order: &order}
	svc := Service{Workspace: deps, Store: deps,
		Hooks: []Hooks{
			testHooks{preDelete: func(models.Workspace) error {
				return &lifecycle.HookError{Hook: "pre_delete", Err: errors.New("warning")}
			}},
			testHooks{preDelete: func(models.Workspace) error { order = append(order, "Go hook"); return nil }},
		},
		Warn: func(message string) { warnings = append(warnings, message) },
	}
	result, err := svc.Delete(DeleteRequest{Name: "login"})
	if err != nil || !result.Deleted || len(warnings) != 1 || warnings[0] != "pre_delete hook failed: warning" {
		t.Fatalf("delete = %+v, %v, warnings = %v", result, err, warnings)
	}
	assertOrder(t, order, "Go hook", "delete")
}

type testHooks struct {
	resolve     func(CreateRequest) (string, error)
	postCreate  func(models.Workspace) error
	postAddRepo func(models.Workspace) error
	preDelete   func(models.Workspace) error
	onClose     func() error
}

func (h testHooks) ResolveBranch(req CreateRequest) (string, error) {
	if h.resolve != nil {
		return h.resolve(req)
	}
	return "", lifecycle.ErrNoHook
}

func (h testHooks) PostCreate(ws models.Workspace) error {
	if h.postCreate != nil {
		return h.postCreate(ws)
	}
	return lifecycle.ErrNoHook
}

func (h testHooks) PostAddRepo(ws models.Workspace) error {
	if h.postAddRepo != nil {
		return h.postAddRepo(ws)
	}
	return lifecycle.ErrNoHook
}

func (h testHooks) PreDelete(ws models.Workspace) error {
	if h.preDelete != nil {
		return h.preDelete(ws)
	}
	return lifecycle.ErrNoHook
}

func (h testHooks) OnClose() error {
	if h.onClose != nil {
		return h.onClose()
	}
	return lifecycle.ErrNoHook
}

// testShellHooks preserves the shell-hook cases while exercising the typed interface.
type testShellHooks struct {
	run     func(string, lifecycle.Vars) error
	resolve func(string, lifecycle.Vars) (string, error)
}

func (h testShellHooks) ResolveBranch(req CreateRequest) (string, error) {
	resolve := h.resolve
	if resolve == nil {
		resolve = lifecycle.Resolve
	}
	return resolve("resolve_branch", lifecycleVars(models.Workspace{Name: req.Name, Branch: req.Options.Branch, Source: req.Options.Source}))
}

func (h testShellHooks) PostCreate(ws models.Workspace) error {
	return h.call("post_create", lifecycleVars(ws))
}
func (h testShellHooks) PostAddRepo(ws models.Workspace) error {
	return h.call("post_add_repo", lifecycleVars(ws))
}
func (h testShellHooks) PreDelete(ws models.Workspace) error {
	return h.call("pre_delete", lifecycleVars(ws))
}
func (h testShellHooks) OnClose() error { return h.call("on_close", lifecycle.Vars{}) }

func (h testShellHooks) call(name string, vars lifecycle.Vars) error {
	run := h.run
	if run == nil {
		run = lifecycle.Run
	}
	return run(name, vars)
}
