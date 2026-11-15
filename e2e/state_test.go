package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfiguredStateDir(t *testing.T) {
	e := newEnv(t)
	e.createRepo("svc-auth")
	e.init()
	e.writeConfig("state_dir = \"~/state\"\n")
	configPath := filepath.Join(e.groveDir, "config.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	e.mustGW("create", "test-ws", "--branch", "feat/e2e", "--repos", "svc-auth")
	if workspaceNamed(e.listWorkspaces(), "test-ws") == nil {
		t.Error("workspace missing from list")
	}
	if got := e.showWorkspace("test-ws").Name; got != "test-ws" {
		t.Errorf("workspace name = %q, want test-ws", got)
	}
	stateDir := filepath.Join(e.home, "state")
	for _, name := range []string{"state.json", "state.lock"} {
		if _, err := os.Stat(filepath.Join(stateDir, name)); err != nil {
			t.Errorf("configured state directory: %s: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(e.groveDir, name)); !os.IsNotExist(err) {
			t.Errorf("unexpected %s in config directory: %v", name, err)
		}
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("workspace commands changed config.toml")
	}
}
