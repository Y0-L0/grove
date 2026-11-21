package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRenameMovesCommittedRecordsWithoutChangingBranch(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		context := filepath.Join(e.workspacePath("a"), filepath.Base(source))
		e.writeFile(filepath.Join(context, "records", "a", "report.md"), "Keep this report\n")
		e.git(context, "add", "records/a/report.md")
		e.git(context, "commit", "--no-gpg-sign", "-m", "Add report")
		e.mustGW("rename", "a", "--to", "b")
		ws := e.showWorkspace("b")
		context = filepath.Join(ws.Path, filepath.Base(source))
		if ws.Branch != e.branch("a") {
			t.Fatalf("workspace branch changed: %s", ws.Branch)
		}
		for _, repo := range ws.Repos {
			if got := e.currentBranch(repo.WorktreePath); got != e.branch("a") {
				t.Errorf("branch = %q", got)
			}
		}
		e.requireContains(e.git(context, "show", "HEAD:records/b/report.md"), "Keep this report", "committed report")
		target, err := os.Readlink(filepath.Join(ws.Path, "records"))
		if err != nil || target != filepath.Join(filepath.Base(source), "records", "b") {
			t.Fatalf("records target %q: %v", target, err)
		}
		e.requireMissing(filepath.Join(context, "records", "a"))
		metadata := e.git(context, "show", "HEAD:records/b/workspace.toml")
		if !strings.Contains(metadata, "name = \"b\"") || !strings.Contains(metadata, e.branch("a")) {
			t.Fatalf("metadata: %s", metadata)
		}
		if got := e.git(context, "status", "--porcelain"); got != "" {
			t.Fatalf("rename left changes: %s", got)
		}
	})
}

func TestCreateRefusesDeletedNameAcrossTypes(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		e.mustGW("delete", "a", "--force")
		before := e.git(source, "rev-parse", "main")
		for _, flags := range [][]string{{}, {"--no-hooks"}} {
			args := append(flags, "create", "a", "--type", "priv", "--repos", "svc-auth", "--branch", e.branch("a"))
			res := e.gw(args...).mustFail(t)
			e.requireContains(res.combined(), "reopen", "deleted name guidance")
			e.requireMissing(filepath.Join(e.wsDir, "priv", "a"))
		}
		if got := e.git(source, "rev-parse", "main"); got != before {
			t.Fatal("guard changed records")
		}
	})
}

func TestReopenRestoresOnlyContextAndRegeneratesRules(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source, _ := e.configureRules("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		original := e.showWorkspace("a")
		e.mustGW("delete", "a", "--force")
		result := e.mustGW("reopen", "a")
		e.requireContains(result.combined(), "gw add-repo 'a'", "former repositories command")
		e.requireContains(result.combined(), "--repos 'svc-auth'", "former repository name")
		ws := e.showWorkspace("a")
		if ws.Branch != original.Branch || ws.Type != "work" || len(ws.Repos) != 1 || !ws.Repos[0].Context {
			t.Fatalf("reopened workspace: %+v", ws)
		}
		context := filepath.Join(ws.Path, filepath.Base(source))
		metadata := e.git(context, "show", "HEAD:records/a/workspace.toml")
		if strings.Contains(metadata, "deleted") || strings.Contains(metadata, "repositories") || !strings.Contains(metadata, original.CreatedAt[:10]) {
			t.Fatalf("reopened metadata: %s", metadata)
		}
		e.requireExists(filepath.Join(ws.Path, "records", "TASK.md"))
		rules, err := os.ReadFile(filepath.Join(ws.Path, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		e.requireContains(string(rules), "Context rules for "+e.user.name, "reopened rules")
		e.requireMissing(filepath.Join(ws.Path, "svc-auth"))
		e.gw("reopen", "a").mustFail(t)
		e.mustGW("delete", "a")
		e.mustGW("reopen", "a")
	})
}

func TestListIncludesActiveAndDeletedRecordsAcrossContexts(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		work := e.configureContext("work")
		priv := e.configureContext("priv")
		e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote(work) + "\n[types.priv]\ncontext = " + strconv.Quote(priv) + "\n")
		e.mustGW("create", "active", "--repos", "svc-auth")
		e.writeFile(filepath.Join(e.workspacePath("active"), "records", "TASK.md"), "# Active\n\n## Goal\nShip the active change.\n\n## State\nIgnore state.\n")
		e.mustGW("create", "deleted", "--type", "priv", "--repos", "svc-auth")
		privateContext := filepath.Join(e.wsDir, "priv", "deleted", filepath.Base(priv))
		e.writeFile(filepath.Join(privateContext, "records", "deleted", "TASK.md"), "# Deleted\n\n## Goal\nKeep the deleted goal.\n")
		e.git(privateContext, "add", "records/deleted/TASK.md")
		e.git(privateContext, "commit", "--no-gpg-sign", "-m", "Write goal")
		e.mustGW("delete", "deleted", "--force")
		entries := decodeJSON[[]struct {
			Name, Status, Goal string
			Repositories       []string
		}](t, e.mustGW("list", "--json").stdout)
		if len(entries) != 2 {
			t.Fatalf("entries: %+v", entries)
		}
		for _, entry := range entries {
			switch entry.Name {
			case "active":
				if entry.Status != "active" || entry.Goal != "Ship the active change." || strings.Join(entry.Repositories, ",") != "svc-auth" {
					t.Errorf("active entry: %+v", entry)
				}
			case "deleted":
				if entry.Status != "deleted" || entry.Goal != "Keep the deleted goal." || strings.Join(entry.Repositories, ",") != "svc-auth" {
					t.Errorf("deleted entry: %+v", entry)
				}
			default:
				t.Errorf("unexpected entry: %+v", entry)
			}
		}
		table := e.mustGW("list").stdout
		e.requireContains(table, "Keep the deleted goal.", "table goal")
		e.requireContains(table, "deleted", "table status")
	})
}

func TestListMatchesSessionsByAnyCwdWithDirectoryBoundaries(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		start := e.home
		transcript := filepath.Join(e.home, ".claude", "projects", "arbitrary-project-name", "match.jsonl")
		var lines []string
		for _, event := range []map[string]any{
			{"sessionId": "match", "cwd": start, "timestamp": "2026-11-18T10:00:00Z"},
			{"sessionId": "match", "cwd": filepath.Join(e.workspacePath("a"), "svc-auth"), "timestamp": "2026-11-18T11:00:00Z"},
			{"sessionId": "match", "aiTitle": "Review the workspace"},
		} {
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, string(data))
		}
		e.writeFile(transcript, strings.Join(lines, "\n")+"\n{unfinished")
		outside, _ := json.Marshal(map[string]any{"sessionId": "outside", "cwd": e.workspacePath("ab"), "timestamp": "2026-11-18T11:00:00Z"})
		e.writeFile(filepath.Join(filepath.Dir(transcript), "outside.jsonl"), string(outside)+"\n")
		old := filepath.Join(filepath.Dir(transcript), "old.jsonl")
		data, _ := json.Marshal(map[string]any{"sessionId": "old", "cwd": e.workspacePath("a")})
		e.writeFile(old, string(data)+"\n")
		if err := os.Chtimes(old, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
		e.mustGW("delete", "a", "--force")
		entries := decodeJSON[[]struct {
			Name     string
			Sessions []struct{ ID, Cwd, ResumeCommand, FirstTimestamp, LastTimestamp, AITitle string }
		}](t, e.mustGW("list", "--json").stdout)
		if len(entries) != 1 || len(entries[0].Sessions) != 1 {
			t.Fatalf("session matches: %+v", entries)
		}
		session := entries[0].Sessions[0]
		if session.ID != "match" {
			t.Fatalf("session id: %s", session.ID)
		}
		var raw []map[string]any
		if err := json.Unmarshal([]byte(e.mustGW("list", "--json").stdout), &raw); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(raw[0]["sessions"])
		for _, expected := range []string{"Review the workspace", "2026-11-18T10:00:00Z", "2026-11-18T11:00:00Z", "claude --resume", start} {
			e.requireContains(string(encoded), expected, "session metadata")
		}
		table := e.mustGW("list").stdout
		e.requireNotContains(table, "claude --resume", "resume command in the list of all workspaces")
		one := e.mustGW("list", "a").stdout
		for _, expected := range []string{"Review the workspace", "2026-11-18T10:00:00Z", "2026-11-18T11:00:00Z", "claude --resume 'match'", start} {
			e.requireContains(one, expected, "sessions of one workspace")
		}
		e.requireNotContains(one, "outside", "session of another workspace")
		e.gw("list", "missing").mustFail(t)
	})
}

func TestListTablePrintsOneLinePerWorkspace(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		e.configureContext("work")
		e.mustGW("create", "multi", "--repos", "svc-auth")
		e.mustGW("create", "single", "--repos", "svc-auth")
		e.writeFile(filepath.Join(e.workspacePath("multi"), "records", "TASK.md"), "# Multi\n\n## Goal\n\nThe first line of the goal\nwraps onto a second line.\n\n| Repo | MR |\n|---|---|\n| svc-auth | !1 |\n\n## State\n")
		e.writeFile(filepath.Join(e.workspacePath("single"), "records", "TASK.md"), "# Single\n\n## Goal\nShip it.\n")
		transcript := filepath.Join(e.home, ".claude", "projects", "any", "session.jsonl")
		event, _ := json.Marshal(map[string]any{"sessionId": "session", "cwd": e.workspacePath("multi"), "timestamp": "2026-11-18T10:00:00Z", "aiTitle": "A session"})
		e.writeFile(transcript, string(event)+"\n")
		lines := strings.Split(strings.TrimRight(e.mustGW("list").stdout, "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("want a header and one line per workspace, got %d lines:\n%s", len(lines), strings.Join(lines, "\n"))
		}
		for i, name := range []string{"multi", "single"} {
			if fields := strings.Fields(lines[i+1]); len(fields) == 0 || fields[0] != name {
				t.Errorf("line %d: want workspace %s, got %q", i+1, name, lines[i+1])
			}
		}
		e.requireContains(lines[0], "SESSIONS", "session count header")
		for i, want := range []string{"1 The first line of the goal", "0 Ship it."} {
			if got := strings.Join(strings.Fields(lines[i+1])[5:], " "); got != want {
				t.Errorf("line %d: want sessions and goal %q, got %q", i+1, want, got)
			}
		}
	})
}

func TestSignFailurePushesNothing(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		source := e.createRepoWithHistory("svc-auth", 1)
		e.init()
		e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		worktree := filepath.Join(e.workspacePath("a"), "svc-auth")
		e.writeFile(filepath.Join(worktree, "change.txt"), "Unsigned change\n")
		e.git(worktree, "add", "change.txt")
		e.git(worktree, "commit", "--no-gpg-sign", "-m", "Unsigned change")
		e.git(source, "config", "gpg.program", "/missing-signing-program")
		origin := filepath.Join(e.home, "svc-auth-origin.git")
		before := e.git(origin, "show-ref")
		res := e.gw("sign", "a").mustFail(t)
		e.requireContains(res.combined(), "signing svc-auth", "signing failure")
		if got := e.git(origin, "show-ref"); got != before {
			t.Fatal("sign failure pushed commits")
		}
	})
}

func (e *env) configureSSHSigning(repo string) {
	e.t.Helper()
	key := filepath.Join(e.home, "signing-key")
	cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	cmd.Env = e.cmdEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		e.t.Fatalf("create signing key: %v: %s", err, out)
	}
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		e.t.Fatal(err)
	}
	allowed := filepath.Join(e.home, "allowed-signers")
	e.writeFile(allowed, e.user.name+"@"+e.user.host+" "+string(public))
	e.git(repo, "config", "gpg.format", "ssh")
	e.git(repo, "config", "user.signingkey", key)
	e.git(repo, "config", "gpg.ssh.allowedSignersFile", allowed)
}

func TestSignPushesVerifiedBranchAndSetsUpstream(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		source := e.createRepoWithHistory("svc-auth", 1)
		origin := filepath.Join(e.home, "svc-auth-origin.git")
		e.git(source, "config", "remote.origin.pushurl", origin)
		e.configureSSHSigning(source)
		e.init()
		context := e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		worktree := filepath.Join(e.workspacePath("a"), "svc-auth")
		e.writeFile(filepath.Join(worktree, "change.txt"), "Signed change\n")
		e.git(worktree, "add", "change.txt")
		e.git(worktree, "commit", "-S", "-m", "Signed change")
		before := e.git(worktree, "rev-parse", "HEAD")
		base := e.git(origin, "rev-parse", "main")
		e.mustGW("sign", "a")
		if got := e.git(origin, "rev-parse", e.branch("a")); got != before {
			t.Fatalf("pushed branch = %s, want %s", got, before)
		}
		if got := e.upstream(worktree); got != "origin/"+e.branch("a") {
			t.Fatalf("upstream = %s", got)
		}
		if got := e.git(origin, "rev-parse", "main"); got != base {
			t.Fatal("sign moved remote main")
		}
		if got := e.git(context, "remote"); got != "" {
			t.Fatal("context gained remote")
		}
		e.writeFile(filepath.Join(worktree, "change.txt"), "Unsigned improvement\n")
		e.git(worktree, "add", "change.txt")
		e.git(worktree, "commit", "--no-gpg-sign", "--fixup", before)
		e.mustGW("sign", "a")
		if got := e.git(worktree, "rev-list", "--count", "origin/main..HEAD"); got != "1" {
			t.Fatalf("fixup not autosquashed: %s", got)
		}
		if got := e.git(worktree, "log", "--format=%G?", "origin/main..HEAD"); got != "G" {
			t.Fatalf("signature status: %s", got)
		}
		if got := e.git(origin, "rev-parse", e.branch("a")); got != e.git(worktree, "rev-parse", "HEAD") {
			t.Fatal("rewritten branch not pushed")
		}
	})
}

func TestRenameRefusesDeletedNamesInOtherContexts(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		work := e.configureContext("work")
		priv := e.configureContext("priv")
		e.writeConfig("owner_prefix = " + strconv.Quote(e.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote(work) + "\n[types.priv]\ncontext = " + strconv.Quote(priv) + "\n")
		e.mustGW("create", "reserved", "--type", "priv", "--repos", "svc-auth")
		e.mustGW("delete", "reserved", "--force")
		e.mustGW("create", "a", "--repos", "svc-auth")
		before := e.mustGW("ws", "show", "a", "--json").stdout
		e.gw("--no-hooks", "rename", "a", "--to", "reserved").mustFail(t)
		if got := e.mustGW("ws", "show", "a", "--json").stdout; got != before {
			t.Fatal("refused rename changed state")
		}
		e.requireExists(filepath.Join(e.workspacePath("a"), "records", "TASK.md"))
	})
}

func TestRecordsCommandCommitFailuresCanBeRetried(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		for _, operation := range []string{"rename", "reopen"} {
			t.Run(operation, func(t *testing.T) {
				e := newEnv(t, e.user)
				e.createRepo("svc-auth")
				e.init()
				source, _ := e.configureRules("priv")
				e.mustGW("create", "a", "--type", "priv", "--repos", "svc-auth")
				if operation == "reopen" {
					e.mustGW("delete", "a", "--force")
				}
				hook := filepath.Join(source, ".git", "hooks", "pre-commit")
				e.writeFile(hook, "#!/bin/sh\necho records-commit-rejected >&2\nexit 1\n")
				if err := os.Chmod(hook, 0o755); err != nil {
					t.Fatal(err)
				}
				args := []string{"--no-hooks", operation, "a"}
				if operation == "rename" {
					args = append(args, "--to", "b")
				}
				res := e.gw(args...).mustFail(t)
				e.requireContains(res.combined(), "records-commit-rejected", "records commit failure")
				ws := e.showWorkspace("a")
				context := filepath.Join(ws.Path, filepath.Base(source))
				if got := e.git(context, "status", "--porcelain"); got != "" {
					t.Fatalf("failed command left changes: %s", got)
				}
				if err := os.Remove(hook); err != nil {
					t.Fatal(err)
				}
				e.mustGW(args...)
				name := "a"
				if operation == "rename" {
					name = "b"
				}
				ws = e.showWorkspace(name)
				e.requireExists(filepath.Join(ws.Path, "records", "TASK.md"))
				if operation == "reopen" {
					e.requireMissing(filepath.Join(ws.Path, "AGENTS.md"))
				}
				e.mustGW("--no-hooks", "delete", name, "--force")
			})
		}
	})
}

func TestRenameSupportsPositionalNewName(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		e.mustGW("rename", "a", "b")
		e.requireExists(filepath.Join(e.workspacePath("b"), "records", "TASK.md"))
	})
}

func TestReopenedWorkspaceKeepsHistoricalSessions(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		source := e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		context := filepath.Join(e.workspacePath("a"), filepath.Base(source))
		path := filepath.Join(context, "records", "a", "workspace.toml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		e.writeFile(path, strings.Replace(string(data), time.Now().Format("2006-01-02"), "2000-01-01", 1))
		e.git(context, "add", "records/a/workspace.toml")
		e.git(context, "commit", "--no-gpg-sign", "-m", "Retain original creation date")
		transcript := filepath.Join(e.home, ".claude", "projects", "history", "past.jsonl")
		event, _ := json.Marshal(map[string]any{"sessionId": "past", "cwd": e.workspacePath("a"), "timestamp": "2001-01-01T00:00:00Z"})
		e.writeFile(transcript, string(event)+"\n")
		past := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(transcript, past, past); err != nil {
			t.Fatal(err)
		}
		e.mustGW("delete", "a", "--force")
		e.mustGW("reopen", "a")
		entries := decodeJSON[[]struct{ Sessions []struct{ ID string } }](t, e.mustGW("list", "--json").stdout)
		if len(entries) != 1 || len(entries[0].Sessions) != 1 || entries[0].Sessions[0].ID != "past" {
			t.Fatalf("historical session missing: %+v", entries)
		}
	})
}

func TestListOrdersSessionTimestampsByInstant(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepo("svc-auth")
		e.init()
		e.configureContext("work")
		e.mustGW("create", "a", "--repos", "svc-auth")
		var lines []string
		for _, stamp := range []string{"2026-11-18T10:00:00+02:00", "2026-11-18T09:00:00Z"} {
			event, _ := json.Marshal(map[string]any{"sessionId": "times", "cwd": e.workspacePath("a"), "timestamp": stamp})
			lines = append(lines, string(event))
		}
		e.writeFile(filepath.Join(e.home, ".claude", "projects", "time", "times.jsonl"), strings.Join(lines, "\n")+"\n")
		entries := decodeJSON[[]struct {
			Sessions []struct {
				First string `json:"first_timestamp"`
				Last  string `json:"last_timestamp"`
			}
		}](t, e.mustGW("list", "--json").stdout)
		if len(entries) != 1 || len(entries[0].Sessions) != 1 {
			t.Fatalf("sessions: %+v", entries)
		}
		session := entries[0].Sessions[0]
		if session.First != "2026-11-18T10:00:00+02:00" || session.Last != "2026-11-18T09:00:00Z" {
			t.Fatalf("timestamp order: %+v", session)
		}
	})
}
