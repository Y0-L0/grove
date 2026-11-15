package e2e

import (
	"path/filepath"
	"strconv"
	"testing"
)

func TestVersion(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		res := env.mustGW("--version")
		if res.stdout == "" && res.stderr == "" {
			t.Fatal("gw --version produced no output")
		}
	})
}

func TestInitAndDoctor(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.mustGW("init", env.reposDir)
		if issues := env.doctorIssues(); len(issues) != 0 {
			t.Fatalf("doctor: found %d issue(s) after clean init: %+v", len(issues), issues)
		}
	})
}

func TestCreateWorkspace(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.createRepo("svc-api")
		env.addGroveTOML("svc-auth", "setup = \"touch .grove-setup-ran\"\n")
		env.init()

		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth,svc-api")

		if workspaceNamed(env.listWorkspaces(), "test-ws") == nil {
			t.Fatal("workspace not in list --json")
		}
		env.requireExists(env.worktree("test-ws", "svc-auth"))
		env.requireExists(env.worktree("test-ws", "svc-api"))
		if got := env.currentBranch(env.worktree("test-ws", "svc-auth")); got != env.branch("feat/e2e") {
			t.Fatalf("expected branch %s, got %s", env.branch("feat/e2e"), got)
		}
		env.requireExists(filepath.Join(env.worktree("test-ws", "svc-auth"), ".grove-setup-ran"))
	})
}

func TestWsListAndShow(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")

		res := env.mustGW("ws", "list", "--json")
		if workspaceNamed(decodeJSON[[]workspaceJSON](t, res.stdout), "test-ws") == nil {
			t.Fatal("gw ws list --json missing workspace")
		}
		shown := env.showWorkspace("test-ws")
		if shown.Name != "test-ws" {
			t.Fatalf("gw ws show --json name = %q", shown.Name)
		}
		env.gw("ws", "show", "nonexistent-ws").mustFail(t)

		// Without a name, ws show uses the workspace containing the cwd.
		sub := env.worktree("test-ws", "svc-auth")
		fromCwd := decodeJSON[workspaceJSON](t, env.mustGWIn(sub, "ws", "show", "--json").stdout)
		if fromCwd.Name != "test-ws" {
			t.Fatalf("gw ws show (no name) from %s: name = %q", sub, fromCwd.Name)
		}
		env.gw("ws", "show").mustFail(t)
	})
}

func TestDuplicateWorkspaceRejected(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")
		env.gw("create", "test-ws", "--branch", env.branch("feat/dupe"), "--repos", "svc-auth").mustFail(t)
	})
}

func TestGo(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")

		res := env.mustGW("go", "test-ws")
		if got := res.stdout; got != env.workspacePath("test-ws") {
			t.Fatalf("go: expected %s, got %s", env.workspacePath("test-ws"), got)
		}
		env.gw("go", "nonexistent-ws").mustFail(t)
	})
}

func TestStatus(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")
		env.mustGW("status", "test-ws")
	})
}

func TestRename(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")
		env.mustGW("rename", "test-ws", "--to", "renamed-ws")

		if workspaceNamed(env.listWorkspaces(), "test-ws") != nil {
			t.Fatal("old workspace name still in list")
		}
		if workspaceNamed(env.listWorkspaces(), "renamed-ws") == nil {
			t.Fatal("new workspace name not in list")
		}
		env.requireExists(env.worktree("renamed-ws", "svc-auth"))
	})
}

func TestMultipleWorkspaces(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")
		env.mustGW("create", "ws-two", "--branch", env.branch("feat/other"), "--repos", "svc-auth")

		if got := len(env.listWorkspaces()); got != 2 {
			t.Fatalf("expected 2 workspaces, got %d", got)
		}
		if got := env.currentBranch(env.worktree("ws-two", "svc-auth")); got != env.branch("feat/other") {
			t.Fatalf("expected %s, got %s", env.branch("feat/other"), got)
		}
	})
}

func TestDeleteWorkspace(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")
		env.mustGW("create", "ws-two", "--branch", env.branch("feat/other"), "--repos", "svc-auth")
		env.mustGW("delete", "ws-two")

		if got := len(env.listWorkspaces()); got != 1 {
			t.Fatalf("expected 1 workspace after delete, got %d", got)
		}
		env.requireMissing(env.workspacePath("ws-two"))
		if env.branchExists(filepath.Join(env.reposDir, "svc-auth"), env.branch("feat/other")) {
			t.Fatalf("branch %s still present in source repo", env.branch("feat/other"))
		}
	})
}

func TestWsDeleteSubcommand(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "keep-ws", "--branch", env.branch("feat/keep"), "--repos", "svc-auth")
		env.mustGW("create", "ws-del-sub", "--branch", env.branch("feat/ws-del"), "--repos", "svc-auth")
		env.mustGW("ws", "delete", "ws-del-sub")

		if got := len(env.listWorkspaces()); got != 1 {
			t.Fatalf("expected 1 workspace after gw ws delete, got %d", got)
		}
		env.requireMissing(env.workspacePath("ws-del-sub"))
	})
}

func TestCreateReplace(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-api")
		env.init()
		env.mustGW("create", "replace-old", "--branch", env.branch("feat/replace-old"), "--repos", "svc-api")
		oldDir := env.workspacePath("replace-old")

		env.mustGWIn(oldDir, "create", "replace-new", "--branch", env.branch("feat/replace-new"), "--repos", "svc-api", "--replace", "-f")

		if workspaceNamed(env.listWorkspaces(), "replace-old") != nil {
			t.Fatal("old workspace still present after --replace")
		}
		if workspaceNamed(env.listWorkspaces(), "replace-new") == nil {
			t.Fatal("new workspace missing after --replace")
		}
		env.requireMissing(oldDir)
		env.requireExists(env.worktree("replace-new", "svc-api"))

		env.gwIn(env.home, "create", "should-not-exist", "--branch", env.branch("feat/nope"), "--repos", "svc-api", "--replace", "-f").mustFail(t)
		env.gwIn(env.workspacePath("replace-new"), "create", "replace-new", "--branch", env.branch("feat/collide"), "--repos", "svc-api", "--replace", "-f").mustFail(t)
	})
}

func TestStatusAutoDetectFromCwd(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")
		env.mustGWIn(env.worktree("test-ws", "svc-auth"), "status")
	})
}

func TestFinalCleanup(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "test-ws", "--branch", env.branch("feat/e2e"), "--repos", "svc-auth")
		env.mustGW("delete", "test-ws")
		if got := len(env.listWorkspaces()); got != 0 {
			t.Fatalf("expected 0 workspaces, got %d", got)
		}
	})
}

func TestCreateResolveBranchHook(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.writeConfig("[hooks]\nresolve_branch = " + strconv.Quote("echo "+env.user.ownerPrefix+"/{branch}") + "\n")

		res := env.mustGW("create", "--branch", "feat-foobar", "--repos", "svc-auth")
		want := env.user.ownerPrefix + "/feat-foobar"
		env.requireContains(res.combined(), "Branch: "+want, "resolved branch notice")
		ws := env.showWorkspace("feat-foobar")
		if ws.Branch != want {
			t.Fatalf("workspace branch = %q, want %q", ws.Branch, want)
		}
		if got := env.currentBranch(env.worktree("feat-foobar", "svc-auth")); got != want {
			t.Fatalf("worktree branch = %q, want %q", got, want)
		}

		// --no-hooks skips the resolver: the branch is used exactly as typed.
		env.mustGW("--no-hooks", "create", "raw-ws", "--branch", "feat-raw", "--repos", "svc-auth")
		if got := env.showWorkspace("raw-ws").Branch; got != "feat-raw" {
			t.Fatalf("--no-hooks branch = %q, want feat-raw", got)
		}
	})
}

func TestCreateResolveBranchFailureCreatesNothing(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.writeConfig(`[hooks]
resolve_branch = "echo no-namer-configured >&2; exit 1"
`)

		res := env.gw("create", "--branch", "feat-x", "--repos", "svc-auth").mustFail(t)
		env.requireContains(res.combined(), "no-namer-configured", "resolver stderr")
		if got := len(env.listWorkspaces()); got != 0 {
			t.Fatalf("expected no workspaces after resolver failure, got %d", got)
		}
	})
}

func TestReplaceKeepsOldWorkspaceWhenResolverFails(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "old-ws", "--branch", "feat-old", "--repos", "svc-auth")
		env.writeConfig(`[hooks]
resolve_branch = "exit 1"
`)

		env.gwIn(env.workspacePath("old-ws"), "create", "new-ws", "--branch", "feat-new",
			"--repos", "svc-auth", "--replace", "--force").mustFail(t)
		env.showWorkspace("old-ws") // fails the test if old-ws was deleted
	})
}
