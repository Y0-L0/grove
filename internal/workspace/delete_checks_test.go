package workspace

import (
	"strings"
	"testing"
)

func TestCheckDeleteReportsEmptyStoredBranchAsCorruptState(t *testing.T) {
	env := setupTestEnv(t)
	env.createRepo("api")
	if err := env.createWorkspace("corrupt-ws", "feat/corrupt", []string{"api"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	ws, _ := env.svc.State.GetWorkspace("corrupt-ws")
	ws.Repos[0].Branch = ""
	if err := env.svc.State.UpdateWorkspace(*ws); err != nil {
		t.Fatalf("corrupt state: %v", err)
	}
	ws, _ = env.svc.State.GetWorkspace("corrupt-ws")

	err := CheckDelete(*ws)

	if err == nil {
		t.Fatal("expected an error for an empty stored branch")
	}
	msg := err.Error()
	if !strings.Contains(msg, "api: corrupt state: workspace corrupt-ws stores no branch for api") {
		t.Errorf("error should name the corrupt state and the repo, got: %s", msg)
	}
	if strings.Contains(msg, "refs/heads/") || strings.Contains(msg, "unexpected") {
		t.Errorf("error should come before any git check runs, got: %s", msg)
	}
}
