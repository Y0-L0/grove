package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nicksenap/grove/internal/models"
)

// The toml package fixes its local-date offset when it loads, so a time zone
// must be set before the test binary starts.
const recordDeletionTZ = "Europe/Berlin"

func TestRecordDeletionKeepsDatesUnderPositiveOffset(t *testing.T) {
	if os.Getenv("TZ") != recordDeletionTZ {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
		cmd.Env = append(os.Environ(), "TZ="+recordDeletionTZ)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("under TZ=%s: %v\n%s", recordDeletionTZ, err, out)
		}
		return
	}

	repoPath := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test")
	path := filepath.Join(repoPath, "records", "drift", "workspace.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	record := "name = \"drift\"\nbranch = \"jlohmer/drift\"\ntype = \"work\"\ngoal = \"TASK.md#goal\"\ncreated = 2026-11-09\n"
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "records")

	ws := models.Workspace{Name: "drift", Repos: []models.RepoWorktree{
		{RepoName: "umbrella", Context: true, WorktreePath: repoPath},
		{RepoName: "api"},
	}}
	repo := ws.Repos[0]
	today := time.Now().Format("2006-01-02")
	for run := 1; run <= 2; run++ {
		if err := recordDeletion(ws, repo); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"created = 2026-11-09\n", "deleted = " + today + "\n"} {
			if !strings.Contains(string(data), want) {
				t.Errorf("run %d: record lacks %q:\n%s", run, want, data)
			}
		}
	}
}
