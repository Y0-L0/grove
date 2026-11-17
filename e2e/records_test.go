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

func (e *env) configureContext(kind string) string {
	e.t.Helper()
	source := filepath.Join(e.home, "context", e.user.name+"-"+kind)
	e.writeFile(filepath.Join(source, "notes.md"), "Context notes\n")
	e.git(source, "init")
	e.git(source, "add", "notes.md")
	e.git(source, "commit", "--no-gpg-sign", "-m", "Initial context")
	e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types." + kind + "]\ncontext = " + strconv.Quote(source) + "\n")
	return source
}

func TestContextRecordsPersistedUnsigned(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "0")
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("priv")
		e.git(source, "config", "commit.gpgsign", "true")
		e.git(source, "config", "gpg.program", "/missing-signing-program")
		e.mustGW("create", "home", "--type", "priv", "--repos", "svc-auth")
		ws := e.showWorkspace("home")
		if len(ws.Repos) != 2 {
			t.Fatalf("persisted repositories = %+v, want code and context", ws.Repos)
		}
		var context repoWorktree
		for _, repo := range ws.Repos {
			if repo.Context {
				context = repo
			} else if repo.RepoName != "svc-auth" {
				t.Errorf("unexpected code repository: %+v", repo)
			}
		}
		if context.SourceRepo != source || context.RepoName != filepath.Base(source) || context.Branch != e.branch("home") {
			t.Fatalf("persisted context repository = %+v", context)
		}
		metadata := e.git(context.WorktreePath, "show", "HEAD:records/home/workspace.toml")
		var entry struct {
			Name, Branch, Type, Goal string
			Created                  time.Time
		}
		decoded, err := toml.Decode(metadata, &entry)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name != "home" || entry.Branch != e.branch("home") || entry.Type != "priv" || entry.Goal != "TASK.md#goal" || entry.Created.Format("2006-01-02") != ws.CreatedAt[:10] {
			t.Fatalf("workspace metadata = %+v", entry)
		}
		if len(decoded.Undecoded()) != 0 {
			t.Fatalf("unexpected active metadata fields: %v", decoded.Undecoded())
		}
		files := e.git(context.WorktreePath, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
		if files != "records/home/TASK.md\nrecords/home/workspace.toml" {
			t.Fatalf("records commit files = %q", files)
		}
		e.requireContains(e.git(context.WorktreePath, "show", "HEAD:records/home/TASK.md"), "## Goal", "committed goal section")
		if raw := e.git(context.WorktreePath, "cat-file", "commit", "HEAD"); strings.Contains(raw, "\ngpgsig ") {
			t.Fatal("records commit is signed")
		}
		if got := e.git(context.WorktreePath, "status", "--porcelain"); got != "" {
			t.Fatalf("dirty records: %s", got)
		}
	})
}

func TestExistingRecordsAreNotOverwritten(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		e.git(source, "checkout", "-b", e.branch("login"))
		e.writeFile(filepath.Join(source, "records", "login", "TASK.md"), "Existing goal\n")
		e.git(source, "add", "records/login/TASK.md")
		e.git(source, "commit", "--no-gpg-sign", "-m", "Existing records")
		before := e.git(source, "rev-parse", "HEAD")
		e.git(source, "checkout", "main")
		res := e.gw("create", "login", "--repos", "svc-auth").mustFail(t)
		e.requireContains(res.combined(), "records already exist", "existing records error")
		if got := e.git(source, "show", e.branch("login")+":records/login/TASK.md"); got != "Existing goal" {
			t.Fatalf("existing goal changed: %q", got)
		}
		if got := e.git(source, "rev-parse", e.branch("login")); got != before {
			t.Fatal("existing context branch changed")
		}
	})
}

func TestContextRecordsSymlink(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "svc-auth")
		ws := e.showWorkspace("login")
		contextName := filepath.Base(source)
		target, err := os.Readlink(filepath.Join(ws.Path, "records"))
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(contextName, "records", "login"); target != want {
			t.Fatalf("records target = %q, want %q", target, want)
		}
		e.requireExists(filepath.Join(ws.Path, "records", "TASK.md"))
		if got := e.currentBranch(filepath.Join(ws.Path, contextName)); got != e.branch("login") {
			t.Fatalf("context branch = %q, want %q", got, e.branch("login"))
		}
	})
}

func TestContextCannotBeRemoved(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		config, err := os.ReadFile(filepath.Join(e.groveDir, "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		e.writeFile(filepath.Join(e.groveDir, "config.toml"), string(config)+"\n[hooks]\npre_delete = 'touch \"$HOME/shell-delete\"'\n")
		e.mustGW("create", "login", "--repos", "svc-auth")
		before := e.mustGW("ws", "show", "login", "--json").stdout
		branch := e.git(source, "rev-parse", e.branch("login"))
		for _, args := range [][]string{
			{"remove-repo", "login", "--repos", "svc-auth," + filepath.Base(source), "--force"},
		} {
			res := e.gw(args...).mustFail(t)
			e.requireMissing(filepath.Join(e.home, "shell-delete"))
			e.requireContains(res.combined(), "context", "context protection")
			if got := e.mustGW("ws", "show", "login", "--json").stdout; got != before {
				t.Fatal("refused removal changed workspace state")
			}
			if got := e.git(source, "rev-parse", e.branch("login")); got != branch {
				t.Fatal("refused removal changed context branch")
			}
			e.requireExists(filepath.Join(e.workspacePath("login"), "svc-auth", ".git"))
			e.requireExists(filepath.Join(e.workspacePath("login"), "records", "TASK.md"))
		}
	})
}

func TestContextUsesLocalBaseBranches(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		for _, base := range []string{"main", "stable"} {
			t.Run(base, func(t *testing.T) {
				e := newEnv(t, e.user)
				e.createRepo("svc-auth")
				e.init()
				source := e.configureContext("work")
				if base != "main" {
					e.git(source, "branch", base)
					e.writeFile(filepath.Join(source, ".grove.toml"), "base_branch = \"stable\"\n")
				}
				e.git(source, "checkout", "-b", "notes")
				e.writeFile(filepath.Join(source, "not-base.md"), "Must not enter the workspace\n")
				e.git(source, "add", "not-base.md")
				e.git(source, "commit", "--no-gpg-sign", "-m", "Notes branch only")
				e.mustGW("create", "login", "--repos", "svc-auth")
				context := filepath.Join(e.workspacePath("login"), filepath.Base(source))
				e.requireMissing(filepath.Join(context, "not-base.md"))
				e.git(source, "checkout", base)
				e.writeFile(filepath.Join(source, "new-base.md"), "New base notes\n")
				e.git(source, "add", "new-base.md")
				e.git(source, "commit", "--no-gpg-sign", "-m", "Advance local base")
				status := decodeJSON[struct {
					Repos []struct{ Repo, Ahead, Behind string }
				}](t, e.mustGW("status", "login", "--json").stdout)
				for _, repo := range status.Repos {
					if repo.Repo == filepath.Base(source) && (repo.Ahead != "1" || repo.Behind != "1") {
						t.Errorf("context status before sync = %+v", repo)
					}
				}
				e.mustGW("sync", "login")
				e.requireExists(filepath.Join(context, "new-base.md"))
				if got := e.commitsBehind(context, "refs/heads/"+base); got != "0" {
					t.Fatalf("context behind local %s = %s", base, got)
				}
				if got := e.upstream(context); got != "none" {
					t.Fatalf("context upstream = %q", got)
				}
			})
		}
	})
}

func TestContextPreparedBeforeLaterHooks(t *testing.T) {
	bin := buildGoHookCLI(t)
	forEachUser(t, func(t *testing.T, e *env) {
		e.bin = bin
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		config, err := os.ReadFile(filepath.Join(e.groveDir, "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		e.writeFile(filepath.Join(e.groveDir, "config.toml"), string(config)+"\n[hooks.post_create]\ncommand = 'test -f {path}/records/workspace.toml && test -f {path}/records/TASK.md && touch \"$HOME/shell-created\"'\non_failure = 'abort'\n")
		e.mustGW("create", "login", "--branch", e.branch("login"), "--repos", "svc-auth")
		e.requireExists(filepath.Join(e.home, "shell-created"))
		events, err := os.ReadFile(filepath.Join(e.home, "hook-events"))
		if err != nil {
			t.Fatal(err)
		}
		want := "go:post_create login go/" + e.branch("login") + " svc-auth," + filepath.Base(source) + "\n"
		if string(events) != want {
			t.Fatalf("typed hook context = %q, want %q", events, want)
		}
	})
}

func TestContextPathFromHomeAndNoHooks(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote("~/context/"+filepath.Base(source)+"/") + "\n[hooks]\npost_create = 'exit 99'\n")
		e.mustGW("--no-hooks", "create", "without", "--repos", "svc-auth")
		ws := e.showWorkspace("without")
		if ws.Branch != "without" || len(ws.Repos) != 1 {
			t.Fatalf("hooks-disabled workspace = %+v", ws)
		}
		e.requireMissing(filepath.Join(ws.Path, "records"))
		e.requireMissing(filepath.Join(ws.Path, filepath.Base(source)))
		e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote("~/context/"+filepath.Base(source)+"/") + "\n")
		e.mustGW("create", "with", "--repos", "svc-auth")
		ws = e.showWorkspace("with")
		e.requireExists(filepath.Join(ws.Path, "records", "workspace.toml"))
		for _, repo := range ws.Repos {
			if repo.Context && repo.SourceRepo != source {
				t.Fatalf("source path = %q, want %q", repo.SourceRepo, source)
			}
		}
	})
}

func TestExplicitContextSelectionIsNotDuplicated(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		original := e.configureContext("work")
		source := filepath.Join(e.reposDir, filepath.Base(original))
		if err := os.Rename(original, source); err != nil {
			t.Fatal(err)
		}
		e.git(source, "checkout", "-b", "notes")
		e.writeFile(filepath.Join(source, "not-base.md"), "Not on the base branch\n")
		e.git(source, "add", "not-base.md")
		e.git(source, "commit", "--no-gpg-sign", "-m", "Notes branch only")
		e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote(source) + "\n")
		e.mustGW("create", "login", "--all")
		ws := e.showWorkspace("login")
		if len(ws.Repos) != 2 {
			t.Fatalf("repositories = %+v", ws.Repos)
		}
		count := 0
		for _, repo := range ws.Repos {
			if repo.Context {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("context count = %d, want 1", count)
		}
		e.requireExists(filepath.Join(ws.Path, "records", "workspace.toml"))
		e.requireMissing(filepath.Join(ws.Path, filepath.Base(source), "not-base.md"))
	})
}

func TestReservedContextNameDoesNotCreateBranch(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		original := e.configureContext("work")
		source := filepath.Join(filepath.Dir(original), "records")
		if err := os.Rename(original, source); err != nil {
			t.Fatal(err)
		}
		e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote(source) + "\n[hooks]\npost_create = 'touch \"$HOME/shell-created\"'\n")
		res := e.gw("create", "login", "--repos", "svc-auth").mustFail(t)
		e.requireContains(res.combined(), "reserved", "reserved context name")
		if e.branchExists(source, e.branch("login")) {
			t.Fatal("context branch created for a reserved worktree name")
		}
		e.requireMissing(filepath.Join(e.home, "shell-created"))
	})
}

func TestRecordsCommitFailureKeepsTrackedContext(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		before := e.git(source, "rev-parse", "HEAD")
		hook := filepath.Join(source, ".git", "hooks", "pre-commit")
		e.writeFile(hook, "#!/bin/sh\necho records-commit-rejected >&2\nexit 1\n")
		if err := os.Chmod(hook, 0o755); err != nil {
			t.Fatal(err)
		}
		config, err := os.ReadFile(filepath.Join(e.groveDir, "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		e.writeFile(filepath.Join(e.groveDir, "config.toml"), string(config)+"\n[hooks]\npost_create = 'touch \"$HOME/shell-created\"'\n")
		res := e.gw("create", "login", "--repos", "svc-auth").mustFail(t)
		e.requireContains(res.combined(), "records-commit-rejected", "commit failure")
		e.requireMissing(filepath.Join(e.home, "shell-created"))
		ws := e.showWorkspace("login")
		var found bool
		for _, repo := range ws.Repos {
			if repo.Context {
				found = true
			}
		}
		if !found {
			t.Fatal("context missing from persisted state after commit failure")
		}
		e.requireExists(filepath.Join(ws.Path, "records", "TASK.md"))
		if got := e.git(source, "rev-parse", e.branch("login")); got != before {
			t.Fatal("failed commit moved the workspace branch")
		}
		e.gw("--no-hooks", "delete", "login").mustFail(t)
	})
}

func TestInvalidContextBaseIsRejectedWhenExplicitlySelected(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		original := e.configureContext("work")
		source := filepath.Join(e.reposDir, filepath.Base(original))
		if err := os.Rename(original, source); err != nil {
			t.Fatal(err)
		}
		e.writeFile(filepath.Join(source, ".grove.toml"), "base_branch = 'missing'\n")
		e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote(source) + "\n")
		res := e.gw("create", "login", "--all").mustFail(t)
		e.requireContains(res.combined(), "local base branch", "invalid context base")
		e.requireMissing(filepath.Join(e.workspacePath("login"), "records"))
	})
}

func TestRecordsCommitHookCanInvokeGrove(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.createRepo("svc-api")
		e.init()
		source := e.configureContext("work")
		hook := filepath.Join(source, ".git", "hooks", "pre-commit")
		e.writeFile(hook, "#!/bin/sh\nunset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE\nexec "+strconv.Quote(e.bin)+" --no-hooks add-repo login --repos svc-api\n")
		if err := os.Chmod(hook, 0o755); err != nil {
			t.Fatal(err)
		}
		e.mustGW("create", "login", "--repos", "svc-auth")
		ws := e.showWorkspace("login")
		if len(ws.Repos) != 3 {
			t.Fatalf("repositories after the commit hook = %+v", ws.Repos)
		}
		context := filepath.Join(ws.Path, filepath.Base(source))
		metadata := e.git(context, "show", "HEAD:records/login/workspace.toml")
		if strings.Contains(metadata, "repositories") {
			t.Fatalf("active repositories duplicated in metadata: %s", metadata)
		}
	})
}

func TestDoctorRetainsMissingContextProtection(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "login", "--repos", "svc-auth")
		before := e.mustGW("ws", "show", "login", "--json").stdout
		context := filepath.Join(e.workspacePath("login"), filepath.Base(source))
		moved := filepath.Join(e.home, "moved-context")
		if err := os.Rename(context, moved); err != nil {
			t.Fatal(err)
		}
		e.mustGW("doctor", "--fix")
		if got := e.mustGW("ws", "show", "login", "--json").stdout; got != before {
			t.Fatal("doctor dropped the missing context entry")
		}
		e.gw("--no-hooks", "delete", "login").mustFail(t)
		e.requireExists(filepath.Join(moved, "records", "login", "TASK.md"))
	})
}
