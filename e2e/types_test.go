package e2e

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDefaultBranchRequiresOwnerPrefix(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		res := env.gw("create", "login", "--repos", "svc-auth").mustFail(t)
		env.requireContains(res.combined(), "owner_prefix", "missing prefix")
		env.requireContains(res.combined(), "--branch", "explicit branch alternative")
		if len(env.listWorkspaces()) != 0 {
			t.Fatal("workspace created without an owner prefix")
		}
		env.requireMissing(env.wsDir)
		env.mustGW("create", "login", "--branch", "review/login", "--repos", "svc-auth")
		if got := env.showWorkspace("login").Branch; got != "review/login" {
			t.Fatalf("explicit branch = %q, want review/login", got)
		}
	})
}

func TestTrackRequiresExplicitBranch(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		res := env.gw("create", "login", "--track", "--repos", "svc-auth").mustFail(t)
		env.requireContains(res.combined(), "--track requires --branch", "track branch error")
		env.requireMissing(env.wsDir)
	})
}

func TestNoHooksUsesWorkspaceNameAsBranch(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.writeConfig("owner_prefix = " + strconv.Quote(env.user.ownerPrefix) + "\n[hooks]\nresolve_branch = \"exit 99\"\n")
		env.mustGW("--no-hooks", "create", "review", "--repos", "svc-auth")
		ws := env.showWorkspace("review")
		if ws.Branch != "review" || ws.Type != "work" {
			t.Fatalf("workspace with hooks disabled = %+v", ws)
		}
		if got := env.currentBranch(filepath.Join(ws.Path, "svc-auth")); got != "review" {
			t.Fatalf("worktree branch = %q, want review", got)
		}
		env.writeConfig("")
		env.mustGW("--no-hooks", "create", "unconfigured", "--repos", "svc-auth")
		if got := env.showWorkspace("unconfigured").Branch; got != "unconfigured" {
			t.Fatalf("branch without owner configuration = %q, want unconfigured", got)
		}
	})
}

func TestDefaultWorkspaceBranch(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.writeConfig("owner_prefix = " + strconv.Quote(env.user.ownerPrefix) + "\n")
		env.mustGW("create", "login", "--repos", "svc-auth")
		ws := env.showWorkspace("login")
		if ws.Branch != env.branch("login") {
			t.Fatalf("branch = %q, want %s", ws.Branch, env.branch("login"))
		}
		if got := env.currentBranch(filepath.Join(ws.Path, "svc-auth")); got != env.branch("login") {
			t.Fatalf("worktree branch = %q, want %s", got, env.branch("login"))
		}
	})
}

func TestWorkspaceTypes(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "office", "--branch", "review/office", "--repos", "svc-auth")
		env.mustGW("create", "home", "--type", "priv", "--branch", "review/home", "--repos", "svc-auth")
		for _, want := range []struct{ name, kind, branch string }{
			{"office", "work", "review/office"},
			{"home", "priv", "review/home"},
		} {
			ws := env.showWorkspace(want.name)
			wantPath := filepath.Join(env.wsDir, want.kind, want.name)
			if ws.Type != want.kind || ws.Path != wantPath || ws.Branch != want.branch {
				t.Errorf("workspace = %+v, want type %s, path %s, branch %s", ws, want.kind, wantPath, want.branch)
			}
			env.requireExists(filepath.Join(wantPath, "svc-auth", ".git"))
		}
	})
}

func TestInvalidWorkspaceTypeCreatesNothing(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		for _, kind := range []string{"other", "../outside", ""} {
			res := env.gw("create", "login", "--type", kind, "--branch", "review/login", "--repos", "svc-auth").mustFail(t)
			env.requireContains(res.combined(), "workspace type", "type error")
		}
		if got := env.listWorkspaces(); len(got) != 0 {
			t.Fatalf("workspaces after invalid types = %+v", got)
		}
		env.requireMissing(env.wsDir)
	})
}

func TestWorkspaceNamesAreUniqueAcrossTypes(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.init()
		env.mustGW("create", "login", "--branch", "review/login", "--repos", "svc-auth")
		res := env.gw("create", "login", "--type", "priv", "--branch", "review/duplicate", "--repos", "svc-auth").mustFail(t)
		env.requireContains(res.combined(), "already exists", "duplicate workspace")
		env.requireMissing(filepath.Join(env.wsDir, "priv", "login"))
		ws := env.showWorkspace("login")
		if ws.Type != "work" || ws.Branch != "review/login" {
			t.Fatalf("original workspace changed: %+v", ws)
		}
	})
}

func TestPrivateWorkspaceKeepsTypeAndBranch(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.createRepo("svc-api")
		env.init()
		env.writeConfig("owner_prefix = " + strconv.Quote(env.user.ownerPrefix) + "\n")
		env.mustGW("create", "home", "--type", "priv", "--repos", "svc-auth")
		env.mustGW("add-repo", "home", "--repos", "svc-api")
		env.mustGW("rename", "home", "--to", "renamed")
		ws := env.showWorkspace("renamed")
		wantPath := filepath.Join(env.wsDir, "priv", "renamed")
		if ws.Type != "priv" || ws.Path != wantPath || ws.Branch != env.branch("home") || len(ws.Repos) != 2 {
			t.Fatalf("renamed private workspace = %+v", ws)
		}
		for _, repo := range ws.Repos {
			if got := env.currentBranch(repo.WorktreePath); got != env.branch("home") {
				t.Errorf("%s branch = %q, want %s", repo.RepoName, got, env.branch("home"))
			}
		}
		env.mustGW("delete", "renamed")
		env.requireMissing(wantPath)
	})
}

func TestExistingUntypedWorkspace(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		env.createRepo("svc-auth")
		env.createRepo("svc-api")
		env.init()
		legacyPath := filepath.Join(env.wsDir, "legacy")
		source := filepath.Join(env.reposDir, "svc-auth")
		worktree := filepath.Join(legacyPath, "svc-auth")
		env.git(source, "worktree", "add", "-b", "old/login", worktree, "main")
		data, err := json.Marshal([]workspaceJSON{{
			Name: "legacy", Path: legacyPath, Branch: "old/login", CreatedAt: "2026-01-01T12:00:00.000000",
			Repos: []repoWorktree{{RepoName: "svc-auth", SourceRepo: source, WorktreePath: worktree, Branch: "old/login"}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		env.writeFile(filepath.Join(env.groveDir, "state.json"), string(data))
		env.mustGW("add-repo", "legacy", "--repos", "svc-api")
		ws := env.showWorkspace("legacy")
		if ws.Type != "" || ws.Path != legacyPath || ws.Branch != "old/login" || len(ws.Repos) != 2 {
			t.Fatalf("legacy workspace changed: %+v", ws)
		}
		env.requireExists(filepath.Join(legacyPath, "svc-api", ".git"))
		env.mustGW("delete", "legacy")
		env.requireMissing(legacyPath)
	})
}
