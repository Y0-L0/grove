package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func TestDeletePlainDirectoryAndMergeRecords(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		e.createRepoWithHistory("web", 1)
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "api")
		e.mustGW("add-repo", "login", "--repos", "web")
		ws := e.showWorkspace("login")
		e.writeFile(filepath.Join(ws.Path, "vmtest", "output"), "plain directory\n")
		e.writeFile(filepath.Join(ws.Path, "records", "report.md"), "Keep this report\n")
		context := filepath.Join(ws.Path, filepath.Base(source))
		e.git(context, "add", "records/login/report.md")
		e.git(context, "commit", "--no-gpg-sign", "-m", "Record findings")
		e.writeFile(filepath.Join(source, "base.md"), "advance base\n")
		e.git(source, "add", "base.md")
		e.git(source, "commit", "--no-gpg-sign", "-m", "Advance base")
		e.writeFile(filepath.Join(source, "notes.md"), "uncommitted clone notes\n")
		e.mustGW("--no-hooks", "delete", "login")
		e.requireMissing(ws.Path)
		e.gw("ws", "show", "login", "--json").mustFail(t)
		e.requireContains(e.git(source, "show", "main:records/login/report.md"), "Keep this report", "merged report")
		metadata := e.git(source, "show", "main:records/login/workspace.toml")
		e.requireContains(metadata, "deleted =", "deleted date")
		e.requireContains(metadata, "\"api\"", "saved repositories")
		e.requireContains(metadata, "\"web\"", "added repository")
		if e.branchExists(source, e.branch("login")) {
			t.Fatal("context branch remains after merge")
		}
		data, err := os.ReadFile(filepath.Join(source, "notes.md"))
		if err != nil || string(data) != "uncommitted clone notes\n" {
			t.Fatalf("clone notes changed: %q, %v", data, err)
		}
	})
}

func TestDeleteForcePreservesOnlyCommittedRecords(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "0")
	forEachUser(t, func(t *testing.T, e *env) {
		api := e.createRepoWithHistory("api", 1)
		e.init()
		source := e.configureContext("priv")
		e.mustGW("create", "home", "--type", "priv", "--repos", "api")
		ws := e.showWorkspace("home")
		context := filepath.Join(ws.Path, filepath.Base(source))
		e.writeFile(filepath.Join(context, "records", "home", "report.md"), "committed report\n")
		e.git(context, "add", "records/home/report.md")
		e.git(context, "commit", "--no-gpg-sign", "-m", "Findings")
		e.writeFile(filepath.Join(context, "records", "home", "report.md"), "dirty report\n")
		e.git(context, "add", "records/home/report.md")
		e.writeFile(filepath.Join(context, "records", "home", "scratch"), "untracked\n")
		e.writeFile(filepath.Join(context, "records", "home", "workspace.toml"), "broken dirty metadata\n")
		e.git(filepath.Join(ws.Path, "api"), "commit", "--no-gpg-sign", "--allow-empty", "-m", "Unpushed code")
		e.writeFile(filepath.Join(ws.Path, "api", "scratch"), "dirty code\n")
		e.git(source, "config", "commit.gpgsign", "true")
		e.git(source, "config", "gpg.program", "/missing-signing-program")
		e.writeFile(filepath.Join(source, "base.md"), "advance base\n")
		e.git(source, "add", "base.md")
		e.git(source, "commit", "--no-gpg-sign", "-m", "Advance base")
		e.mustGW("--no-hooks", "ws", "delete", "home", "--force")
		e.requireMissing(ws.Path)
		if e.branchExists(api, e.branch("home")) || e.branchExists(source, e.branch("home")) {
			t.Fatal("workspace branch remains")
		}
		if got := e.git(source, "show", "main:records/home/report.md"); got != "committed report" {
			t.Fatalf("forced deletion merged dirty records: %q", got)
		}
		e.requireMissing(filepath.Join(source, "records", "home", "scratch"))
		var entry struct {
			Deleted      time.Time
			Repositories []string
		}
		metadata := e.git(source, "show", "main:records/home/workspace.toml")
		if _, err := toml.Decode(metadata, &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Deleted.IsZero() || len(entry.Repositories) != 1 || entry.Repositories[0] != "api" {
			t.Fatalf("deleted metadata = %+v", entry)
		}
		if raw := e.git(source, "cat-file", "commit", "HEAD"); strings.Contains(raw, "\ngpgsig ") {
			t.Fatal("delete commit was signed")
		}
	})
}

func TestDeleteRejectsUnexpectedAndUnregisteredWorktrees(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		api := e.createRepoWithHistory("api", 1)
		web := e.createRepoWithHistory("web", 1)
		e.init()
		e.writeConfig("owner_prefix = '" + e.user.ownerPrefix + "'\n")
		e.mustGW("create", "login", "--repos", "api,web")
		ws := e.showWorkspace("login")
		e.git(filepath.Join(ws.Path, "api"), "switch", "-c", "other")
		e.git(web, "worktree", "remove", filepath.Join(ws.Path, "web"))
		e.writeFile(filepath.Join(ws.Path, "web", "ordinary"), "replacement\n")
		before := e.git(api, "worktree", "list", "--porcelain")
		res := e.gw("delete", "login").mustFail(t)
		e.requireContains(res.combined(), "unexpected branch", "branch check")
		e.requireContains(res.combined(), "not registered", "registration check")
		if e.git(api, "worktree", "list", "--porcelain") != before {
			t.Fatal("refusal changed worktrees")
		}
		e.requireExists(filepath.Join(ws.Path, "web", "ordinary"))
		e.showWorkspace("login")
	})
}

func TestDeleteTemporaryWorktreeAloneBlocksRemoval(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		outside := e.createRepo("outside")
		e.init()
		e.writeConfig("owner_prefix = '" + e.user.ownerPrefix + "'\n")
		e.mustGW("create", "login", "--repos", "api")
		ws := e.showWorkspace("login")
		temporary := filepath.Join(ws.Path, "outside-review")
		e.git(outside, "worktree", "add", "--detach", temporary)
		before := e.git(outside, "worktree", "list", "--porcelain")
		res := e.gw("delete", "login").mustFail(t)
		e.requireContains(res.combined(), temporary, "temporary worktree")
		if e.git(outside, "worktree", "list", "--porcelain") != before {
			t.Fatal("temporary worktree registration changed")
		}
		e.showWorkspace("login")
		e.git(temporary, "worktree", "remove", temporary)
		e.mustGW("delete", "login")
	})
}

func TestDeleteRetryAfterRecordsMerge(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "api")
		config, err := os.ReadFile(filepath.Join(e.groveDir, "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		e.writeFile(filepath.Join(e.groveDir, "config.toml"), string(config)+"\n[hooks.pre_delete]\ncommand = 'exit 1'\non_failure = 'abort'\n")
		e.gw("delete", "login").mustFail(t)
		e.showWorkspace("login")
		merged := e.git(source, "rev-parse", "main")
		e.requireContains(e.git(source, "show", "main:records/login/workspace.toml"), "deleted =", "merged before shell failure")
		e.mustGW("--no-hooks", "delete", "login")
		if e.git(source, "rev-parse", "main") != merged {
			t.Fatal("retry created another records commit")
		}
	})
}

func TestPruneChecksAndMergesContextRecords(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "api")
		e.writeFile(filepath.Join(e.workspacePath("login"), "records", "scratch"), "untracked\n")
		e.gw("--no-hooks", "prune", "--min-age", "0", "--yes").mustFail(t)
		e.showWorkspace("login")
		if err := os.Remove(filepath.Join(e.workspacePath("login"), "records", "scratch")); err != nil {
			t.Fatal(err)
		}
		e.mustGW("--no-hooks", "prune", "--min-age", "0", "--yes")
		e.requireContains(e.git(source, "show", "main:records/login/workspace.toml"), "deleted =", "pruned records")
	})
}

func TestDeleteCommitFailureCanBeRetried(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "api")
		context := filepath.Join(e.workspacePath("login"), filepath.Base(source))
		branch := e.git(context, "rev-parse", "HEAD")
		base := e.git(source, "rev-parse", "main")
		hook := filepath.Join(source, ".git", "hooks", "pre-commit")
		e.writeFile(hook, "#!/bin/sh\necho delete-commit-rejected >&2\nexit 1\n")
		if err := os.Chmod(hook, 0o755); err != nil {
			t.Fatal(err)
		}
		res := e.gw("delete", "login").mustFail(t)
		e.requireContains(res.combined(), "delete-commit-rejected", "commit rejection")
		e.showWorkspace("login")
		if e.git(context, "rev-parse", "HEAD") != branch || e.git(source, "rev-parse", "main") != base {
			t.Fatal("failed commit changed branches")
		}
		if got := e.git(context, "status", "--porcelain"); got != "" {
			t.Fatalf("failed metadata commit left changes: %s", got)
		}
		if err := os.Remove(hook); err != nil {
			t.Fatal(err)
		}
		e.mustGW("delete", "login")
	})
}

func TestDeleteMergeConflictPreservesWorkspace(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "api")
		context := filepath.Join(e.workspacePath("login"), filepath.Base(source))
		e.writeFile(filepath.Join(context, "notes.md"), "workspace notes\n")
		e.git(context, "add", "notes.md")
		e.git(context, "commit", "--no-gpg-sign", "-m", "Workspace notes")
		e.writeFile(filepath.Join(source, "notes.md"), "base notes\n")
		e.git(source, "add", "notes.md")
		e.git(source, "commit", "--no-gpg-sign", "-m", "Base notes")
		base := e.git(source, "rev-parse", "main")
		res := e.gw("delete", "login").mustFail(t)
		e.requireContains(res.combined(), "CONFLICT", "rebase conflict")
		e.showWorkspace("login")
		e.requireExists(filepath.Join(e.workspacePath("login"), "api", ".git"))
		if e.currentBranch(context) != e.branch("login") || e.git(source, "rev-parse", "main") != base {
			t.Fatal("failed rebase changed base or left a detached checkout")
		}
		if got := e.git(context, "status", "--porcelain"); got != "" {
			t.Fatalf("rebase was not aborted: %s", got)
		}
		// Resolve the conflict through a new base commit, then retry the same delete.
		e.writeFile(filepath.Join(source, "notes.md"), "workspace notes\n")
		e.git(source, "add", "notes.md")
		e.git(source, "commit", "--no-gpg-sign", "-m", "Accept workspace notes")
		e.mustGW("delete", "login")
	})
}

func TestDeleteRecoversMissingContextCheckout(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		e.init()
		source := e.configureContext("work")
		e.git(source, "branch", "stable")
		e.writeFile(filepath.Join(source, ".grove.toml"), "base_branch = 'stable'\n")
		e.git(source, "switch", "stable")
		e.mustGW("create", "login", "--repos", "api")
		ws := e.showWorkspace("login")
		if err := os.RemoveAll(ws.Path); err != nil {
			t.Fatal(err)
		}
		e.mustGW("delete", "login")
		e.requireMissing(ws.Path)
		e.requireContains(e.git(source, "show", "stable:records/login/workspace.toml"), "deleted =", "recovered records")
		if e.currentBranch(source) != "stable" {
			t.Fatal("delete changed the configured base branch")
		}
	})
}

func TestDeleteChecksCommitsWithoutRequiringUpstream(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		api := e.createRepoWithHistory("api", 1)
		e.init()
		e.writeConfig("owner_prefix = '" + e.user.ownerPrefix + "'\n")
		e.mustGW("create", "login", "--repos", "api")
		worktree := filepath.Join(e.workspacePath("login"), "api")
		e.git(worktree, "commit", "--no-gpg-sign", "--allow-empty", "-m", "Publish under another name")
		res := e.gw("delete", "login").mustFail(t)
		e.requireContains(res.combined(), "unpushed", "unpushed commit")
		e.git(api, "remote", "add", "archive", filepath.Join(e.home, "api-origin.git"))
		e.git(worktree, "push", "archive", "HEAD:published")
		if e.upstream(worktree) != "none" {
			t.Fatal("test branch has an upstream")
		}
		e.mustGW("delete", "login")
		if e.branchExists(api, e.branch("login")) {
			t.Fatal("published workspace branch remains")
		}
	})
}

func TestDeleteStopsIfCommitHookAddsARepository(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("api", 1)
		e.createRepoWithHistory("web", 1)
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "api")
		hook := filepath.Join(source, ".git", "hooks", "pre-commit")
		e.writeFile(hook, "#!/bin/sh\nunset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE\nexec "+strconv.Quote(e.bin)+" --no-hooks add-repo login --repos web\n")
		if err := os.Chmod(hook, 0o755); err != nil {
			t.Fatal(err)
		}
		res := e.gw("delete", "login").mustFail(t)
		e.requireContains(res.combined(), "changed after pre-delete checks", "changed repositories")
		ws := e.showWorkspace("login")
		if len(ws.Repos) != 3 {
			t.Fatalf("repository added by hook is missing: %+v", ws.Repos)
		}
		e.requireExists(filepath.Join(ws.Path, "api", ".git"))
		e.requireExists(filepath.Join(ws.Path, "web", ".git"))
		e.mustGW("delete", "login")
		e.requireContains(e.git(source, "show", "main:records/login/workspace.toml"), "\"web\"", "updated retry metadata")
	})
}

func TestDeleteReportsAllProblemsWithoutChanges(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		api := e.createRepoWithHistory("api", 1)
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "api")
		ws := e.showWorkspace("login")
		context := filepath.Join(ws.Path, filepath.Base(source))
		e.git(filepath.Join(ws.Path, "api"), "commit", "--no-gpg-sign", "--allow-empty", "-m", "Local work")
		e.writeFile(filepath.Join(ws.Path, "api", "untracked"), "keep code\n")
		e.writeFile(filepath.Join(context, "records", "login", "untracked"), "keep records\n")
		temporary := filepath.Join(ws.Path, "api-review")
		e.git(api, "worktree", "add", "--detach", temporary)
		e.git(source, "checkout", "-b", "notes")
		before := e.mustGW("ws", "show", "login", "--json").stdout
		contextHead := e.git(context, "rev-parse", "HEAD")
		codeHead := e.git(api, "rev-parse", e.branch("login"))
		res := e.gw("--no-hooks", "delete", "login").mustFail(t)
		for _, problem := range []string{"api", "dirty", "unpushed", "api-review", "worktree remove", "base branch"} {
			e.requireContains(res.combined(), problem, "delete problems")
		}
		if got := e.mustGW("ws", "show", "login", "--json").stdout; got != before {
			t.Fatal("refusal changed state")
		}
		if e.git(context, "rev-parse", "HEAD") != contextHead || e.git(api, "rev-parse", e.branch("login")) != codeHead {
			t.Fatal("refusal changed branches")
		}
		e.requireExists(filepath.Join(temporary, ".git"))
		e.requireExists(filepath.Join(context, "records", "login", "untracked"))
		e.requireExists(filepath.Join(ws.Path, "api", "untracked"))
	})
}
