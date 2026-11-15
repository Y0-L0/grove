package e2e

import (
	"path/filepath"
	"testing"
)

func TestUserConfiguration(t *testing.T) {
	forEachUser(t, func(t *testing.T, e *env) {
		e.createRepoWithHistory("svc-auth", 1)
		e.init()
		e.writeConfig(`[hooks.post_create]
command = "echo user=$USER logname=$LOGNAME; git -C {path}/svc-auth log -1 --format=author=%an,email=%ae"
stream = true
`)

		out := e.mustGW("create", "profile-ws", "--branch", e.branch("feature"), "--repos", "svc-auth")
		e.requireContains(out.combined(), "user="+e.user.name+" logname="+e.user.name, "user environment")
		e.requireContains(out.combined(), "author="+e.user.name+",email="+e.user.name+"@"+e.user.host, "Git identity")
		ws := e.showWorkspace("profile-ws")
		if want := filepath.Join(e.home, e.user.workspaceDir, "profile-ws"); ws.Path != want {
			t.Errorf("workspace path = %q, want %q", ws.Path, want)
		}
		if want := e.user.ownerPrefix + "/feature"; ws.Branch != want {
			t.Errorf("workspace branch = %q, want %q", ws.Branch, want)
		}
		repos := decodeJSON[[]repoEntry](t, e.mustGW("repos", "--json").stdout)
		if len(repos) != 1 {
			t.Fatalf("got %d repositories, want 1", len(repos))
		}
		if want := filepath.Join(e.home, e.user.repoDir, "svc-auth"); repos[0].Path != want {
			t.Errorf("repository path = %q, want %q", repos[0].Path, want)
		}
		remote := e.git(repos[0].Path, "config", "--get", "remote.origin.url")
		if want := "https://" + e.user.host + "/svc-auth-origin.git"; remote != want {
			t.Errorf("repository remote = %q, want %q", remote, want)
		}
	})
}
