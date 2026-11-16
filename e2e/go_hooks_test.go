package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGoHooksRunBeforeShellHooks(t *testing.T) {
	bin := buildGoHookCLI(t)
	forEachUser(t, func(t *testing.T, e *env) {
		e.bin = bin
		e.createRepoWithHistory("svc-auth", 1)
		e.createRepoWithHistory("svc-api", 1)
		e.init()
		e.writeConfig(`[hooks]
resolve_branch = "echo {branch}-shell"
post_create = "echo shell:post_create >> \"$HOME/hook-events\""
post_add_repo = "echo shell:post_add_repo >> \"$HOME/hook-events\""
pre_delete = "echo shell:pre_delete >> \"$HOME/hook-events\""
`)
		e.mustGW("create", "login", "--branch", e.branch("login"), "--repos", "svc-auth")
		ws := e.showWorkspace("login")
		wantBranch := "go/" + e.user.ownerPrefix + "/login-shell"
		if ws.Branch != wantBranch {
			t.Fatalf("branch = %q, want %q", ws.Branch, wantBranch)
		}
		e.mustGW("add-repo", "login", "--repos", "svc-api")
		e.mustGW("delete", "login")
		events, err := os.ReadFile(filepath.Join(e.home, "hook-events"))
		if err != nil {
			t.Fatal(err)
		}
		want := "go:post_create login " + wantBranch + " svc-auth\n" +
			"shell:post_create\n" +
			"go:post_add_repo login " + wantBranch + " svc-auth,svc-api\n" +
			"shell:post_add_repo\n" +
			"go:pre_delete login " + wantBranch + " svc-auth,svc-api\n" +
			"shell:pre_delete\n"
		if string(events) != want {
			t.Fatalf("hook events:\n%s\nwant:\n%s", events, want)
		}
	})
}

func buildGoHookCLI(t *testing.T) string {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "gw")
	cmd := exec.Command("go", "build", "-o", bin, "./e2e/testdata/gohooks")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build Go hook CLI: %v\n%s", err, out)
	}
	return bin
}

func TestNoHooksSkipsGoAndShellHooks(t *testing.T) {
	bin := buildGoHookCLI(t)
	forEachUser(t, func(t *testing.T, e *env) {
		e.bin = bin
		e.createRepoWithHistory("svc-auth", 1)
		e.createRepoWithHistory("svc-api", 1)
		e.init()
		e.writeConfig(`[hooks]
resolve_branch = "exit 99"
post_create = "exit 99"
post_add_repo = "exit 99"
pre_delete = "exit 99"
`)
		e.mustGW("-n", "create", "login", "--branch", e.branch("login"), "--repos", "svc-auth")
		ws := e.showWorkspace("login")
		if ws.Branch != e.user.ownerPrefix+"/login" {
			t.Fatalf("branch = %q", ws.Branch)
		}
		e.mustGW("-n", "add-repo", "login", "--repos", "svc-api")
		e.mustGW("-n", "delete", "login")
		e.requireMissing(filepath.Join(e.home, "hook-events"))
	})
}

func TestGoHookFailurePreventsDelete(t *testing.T) {
	bin := buildGoHookCLI(t)
	forEachUser(t, func(t *testing.T, e *env) {
		e.bin = bin
		e.createRepoWithHistory("svc-auth", 1)
		e.init()
		e.writeConfig(`[hooks]
pre_delete = "touch \"$HOME/shell-delete\""
`)
		e.mustGW("-n", "create", "login", "--branch", e.branch("login"), "--repos", "svc-auth")
		// A directory at the event-file path makes the Go hook's write fail.
		e.writeFile(filepath.Join(e.home, "hook-events", "blocked"), "")
		out := e.gw("delete", "login").mustFail(t)
		e.requireContains(out.combined(), "hook-events", "Go hook error")
		e.requireMissing(filepath.Join(e.home, "shell-delete"))
		ws := e.showWorkspace("login")
		e.requireExists(filepath.Join(ws.Path, "svc-auth", ".git"))
	})
}
