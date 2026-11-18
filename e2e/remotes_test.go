package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAutomaticRemoteBranchAdoption(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		for _, operation := range []string{"create", "add-repo"} {
			t.Run(operation, func(t *testing.T) {
				env := newEnv(t, env.user)
				repoName := "svc-" + operation
				repo := env.createRepoWithHistory(repoName, 1)
				name := "review-" + operation
				branch := env.branch(name)
				env.pushRemoteBranch(repo, branch, "remote-work.txt")
				origin := filepath.Join(env.home, repoName+"-origin.git")
				wantCommit := env.git(origin, "rev-parse", "refs/heads/"+branch)
				env.init()
				env.writeConfig("owner_prefix = " + strconv.Quote(env.user.ownerPrefix) + "\n")
				if operation == "add-repo" {
					env.createRepo("seed")
					env.mustGW("create", name, "--repos", "seed")
					env.mustGW("add-repo", name, "--repos", repoName)
				} else {
					env.mustGW("create", name, "--repos", repoName)
				}
				path := filepath.Join(env.showWorkspace(name).Path, repoName)
				if got := env.git(path, "rev-parse", "HEAD"); got != wantCommit {
					t.Errorf("HEAD = %s, want remote commit %s", got, wantCommit)
				}
				if got := env.upstream(path); got != "origin/"+branch {
					t.Errorf("upstream = %s, want origin/%s", got, branch)
				}
			})
		}
	})
}

func TestRemotePolicyPreservesExistingConfiguration(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		existing := env.createRepoWithHistory("existing", 1)
		local := env.createRepoWithHistory("local", 1)
		ssh := env.createRepoWithHistory("ssh", 1)
		env.git(existing, "config", "--add", "remote.origin.pushurl", "ssh://custom.example/first.git")
		env.git(existing, "config", "--add", "remote.origin.pushurl", "ssh://custom.example/second.git")
		env.git(local, "remote", "set-url", "origin", filepath.Join(env.home, "local-origin.git"))
		sshURL := "git@" + env.user.host + ":ssh-origin.git"
		env.git(env.home, "config", "--global", "--add", "url."+fileURL(env.home)+"/.insteadOf", "git@"+env.user.host+":")
		env.git(ssh, "remote", "set-url", "origin", sshURL)
		env.init()
		env.mustGW("create", "preserve", "--branch", env.branch("preserve"), "--repos", "existing,local,ssh")
		if got := env.git(existing, "config", "--get-all", "remote.origin.pushurl"); got != "ssh://custom.example/first.git\nssh://custom.example/second.git" {
			t.Errorf("existing push URLs changed: %q", got)
		}
		for _, repo := range []string{local, ssh} {
			if got, err := env.gitAllowFail(repo, "config", "--get", "remote.origin.pushurl"); err == nil || got != "" {
				t.Errorf("non-HTTPS origin acquired a push URL: %q, %v", got, err)
			}
		}
	})
}

func TestNoHooksStillAdoptsRemoteBranches(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		first := env.createRepoWithHistory("svc-auth", 1)
		second := env.createRepoWithHistory("svc-api", 1)
		branch := env.branch("disabled")
		for _, repo := range []string{first, second} {
			env.pushRemoteBranch(repo, branch, "remote-work.txt")
		}
		env.init()
		env.mustGW("--no-hooks", "create", "disabled", "--branch", branch, "--repos", "svc-auth")
		env.mustGW("--no-hooks", "add-repo", "disabled", "--repos", "svc-api")
		for _, name := range []string{"svc-auth", "svc-api"} {
			path := env.worktree("disabled", name)
			env.requireExists(filepath.Join(path, "remote-work.txt"))
			if got := env.upstream(path); got != "origin/"+branch {
				t.Errorf("upstream = %q, want origin/%s", got, branch)
			}
			if got, err := env.gitAllowFail(path, "config", "--get", "remote.origin.pushurl"); err == nil || got != "" {
				t.Errorf("disabled hooks wrote a push URL: %q, %v", got, err)
			}
		}
	})
}

func TestExistingLocalBranchWinsOverRemote(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		for _, operation := range []string{"create", "add-repo"} {
			t.Run(operation, func(t *testing.T) {
				env := newEnv(t, env.user)
				repo := env.createRepoWithHistory("svc-auth", 1)
				branch := env.branch("local")
				env.pushRemoteBranch(repo, branch, "remote-work.txt")
				env.git(repo, "checkout", "-b", branch, "--track", "origin/main")
				env.git(repo, "commit", "--allow-empty", "-m", "local work")
				want := env.git(repo, "rev-parse", "HEAD")
				env.git(repo, "checkout", "main")
				env.init()
				if operation == "create" {
					env.mustGW("create", "local", "--branch", branch, "--repos", "svc-auth")
				} else {
					env.createRepo("seed")
					env.mustGW("create", "local", "--branch", branch, "--repos", "seed")
					env.mustGW("add-repo", "local", "--repos", "svc-auth")
				}
				path := env.worktree("local", "svc-auth")
				if got := env.git(path, "rev-parse", "HEAD"); got != want {
					t.Errorf("existing branch moved to %s, want %s", got, want)
				}
				if got := env.upstream(path); got != "origin/main" {
					t.Errorf("existing upstream changed to %q", got)
				}
			})
		}
	})
}

func TestAdoptionRollsBackWhenAnotherRepoFails(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		for _, operation := range []string{"create", "add-repo"} {
			t.Run(operation, func(t *testing.T) {
				env := newEnv(t, env.user)
				first := env.createRepoWithHistory("svc-auth", 1)
				second := env.createRepoWithHistory("svc-api", 1)
				branch := env.branch("rollback")
				env.pushRemoteBranch(first, branch, "remote-work.txt")
				env.git(second, "checkout", "-b", branch)
				want := env.git(second, "rev-parse", "HEAD")
				env.init()
				if operation == "create" {
					env.gw("create", "rollback", "--branch", branch, "--repos", "svc-auth,svc-api").mustFail(t)
					env.requireMissing(env.workspacePath("rollback"))
					if len(env.listWorkspaces()) != 0 {
						t.Error("failed create left workspace state")
					}
				} else {
					env.createRepo("seed")
					env.mustGW("create", "rollback", "--branch", branch, "--repos", "seed")
					env.gw("add-repo", "rollback", "--repos", "svc-auth,svc-api").mustFail(t)
					if ws := env.showWorkspace("rollback"); len(ws.Repos) != 1 || ws.Repos[0].RepoName != "seed" {
						t.Errorf("failed add-repo changed workspace: %+v", ws)
					}
				}
				if env.branchExists(first, branch) {
					t.Error("rollback retained newly adopted local branch")
				}
				if got := env.git(second, "rev-parse", branch); got != want {
					t.Errorf("existing branch moved to %s, want %s", got, want)
				}
				env.git(first, "rev-parse", "--verify", "refs/remotes/origin/"+branch)
				env.requireMissing(env.worktree("rollback", "svc-auth"))
			})
		}
	})
}

func TestPushURLFailureKeepsTrackedWorktrees(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		for _, operation := range []string{"create", "add-repo"} {
			t.Run(operation, func(t *testing.T) {
				env := newEnv(t, env.user)
				repo := env.createRepoWithHistory("svc-auth", 1)
				env.init()
				if operation == "add-repo" {
					env.createRepo("seed")
					env.mustGW("create", "locked", "--branch", env.branch("locked"), "--repos", "seed")
				}
				marker := filepath.Join(env.home, "shell-ran")
				env.writeConfig("[hooks]\npost_create = " + strconv.Quote("touch "+marker) + "\npost_add_repo = " + strconv.Quote("touch "+marker) + "\n")
				env.writeFile(filepath.Join(repo, ".git", "config.lock"), "locked\n")
				var res cmdResult
				if operation == "create" {
					res = env.gw("create", "locked", "--branch", env.branch("locked"), "--repos", "svc-auth").mustFail(t)
				} else {
					res = env.gw("add-repo", "locked", "--repos", "svc-auth").mustFail(t)
				}
				env.requireContains(res.combined(), "configuring push URL for svc-auth", "configuration failure")
				env.requireMissing(marker)
				ws := env.showWorkspace("locked")
				found := false
				for _, r := range ws.Repos {
					if r.RepoName == "svc-auth" {
						found = true
						env.requireExists(filepath.Join(r.WorktreePath, ".git"))
					}
				}
				if !found {
					t.Error("post-hook failure left the worktree untracked")
				}
				if err := os.Remove(filepath.Join(repo, ".git", "config.lock")); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
}

func TestRemotePolicyExcludesContextRepository(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		source := env.createRepoWithHistory("context", 1)
		env.createRepoWithHistory("svc-auth", 1)
		env.createRepoWithHistory("svc-api", 1)
		env.init()
		env.writeConfig("owner_prefix = " + strconv.Quote(env.user.ownerPrefix) + "\n[types.work]\ncontext = " + strconv.Quote(source) + "\n")
		env.mustGW("create", "contextual", "--repos", "context,svc-auth")
		env.mustGW("add-repo", "contextual", "--repos", "svc-api")
		if got, err := env.gitAllowFail(source, "config", "--get", "remote.origin.pushurl"); err == nil || got != "" {
			t.Errorf("context acquired a push URL: %q, %v", got, err)
		}
	})
}

func TestPushURLFollowsFirstConfiguredFetchURL(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		repo := env.createRepoWithHistory("svc-auth", 1)
		origin := filepath.Join(env.home, "svc-auth-origin.git")
		url := "https://" + env.user.host + "/team/service"
		env.git(env.home, "config", "--global", "url."+fileURL(origin)+".insteadOf", url)
		env.git(repo, "config", "remote.origin.url", url)
		env.git(repo, "config", "--add", "remote.origin.url", "https://other.invalid/other.git")
		env.init()
		env.mustGW("create", "nested", "--branch", env.branch("nested"), "--repos", "svc-auth")
		if got, want := env.git(repo, "config", "--get", "remote.origin.pushurl"), "git@"+env.user.host+":team/service.git"; got != want {
			t.Errorf("push URL = %q, want %q", got, want)
		}
	})
}

func TestFreshWorkspaceBranchesHaveNoUpstream(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		first := env.createRepoWithHistory("svc-auth", 1)
		second := env.createRepoWithHistory("svc-api", 1)
		env.git(first, "config", "branch.autoSetupMerge", "always")
		env.git(second, "config", "branch.autoSetupMerge", "always")
		env.init()
		env.mustGW("create", "fresh", "--branch", env.branch("fresh"), "--repos", "svc-auth")
		env.mustGW("add-repo", "fresh", "--repos", "svc-api")
		for _, name := range []string{"svc-auth", "svc-api"} {
			path := env.worktree("fresh", name)
			if got := env.upstream(path); got != "none" {
				t.Errorf("%s upstream = %s, want none", name, got)
			}
			if got, want := env.git(path, "rev-parse", "HEAD"), env.git(path, "rev-parse", "origin/main"); got != want {
				t.Errorf("%s HEAD = %s, want base commit %s", name, got, want)
			}
		}
	})
}

func TestWorkspacePushDoesNotUpdateMain(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "0")
	forEachUser(t, func(t *testing.T, env *env) {
		for _, operation := range []string{"create", "add-repo", "track-fallback"} {
			t.Run(operation, func(t *testing.T) {
				env := newEnv(t, env.user)
				repo := env.createRepoWithHistory("svc-auth", 1)
				origin := filepath.Join(env.home, "svc-auth-origin.git")
				mainBefore := env.git(origin, "rev-parse", "refs/heads/main")
				env.git(repo, "config", "branch.autoSetupMerge", "true")
				env.git(repo, "config", "push.default", "upstream")
				env.git(repo, "config", "push.autoSetupRemote", "true")
				env.init()
				branch := env.branch("safe-push")
				if operation == "add-repo" {
					env.createRepo("seed")
					env.mustGW("--no-hooks", "create", "safe-push", "--branch", branch, "--repos", "seed")
					env.mustGW("--no-hooks", "add-repo", "safe-push", "--repos", "svc-auth")
				} else {
					args := []string{"--no-hooks", "create", "safe-push", "--branch", branch, "--repos", "svc-auth"}
					if operation == "track-fallback" {
						args = append(args, "--track")
					}
					env.mustGW(args...)
				}
				worktree := env.worktree("safe-push", "svc-auth")
				env.writeFile(filepath.Join(worktree, "workspace.txt"), "workspace work\n")
				env.git(worktree, "add", "workspace.txt")
				env.git(worktree, "commit", "-q", "-m", "workspace work")
				want := env.git(worktree, "rev-parse", "HEAD")
				env.git(worktree, "push")
				if got := env.git(origin, "rev-parse", "refs/heads/main"); got != mainBefore {
					t.Errorf("plain git push changed remote main from %s to %s", mainBefore, got)
				}
				if got, err := env.gitAllowFail(origin, "rev-parse", "--verify", "refs/heads/"+branch); err != nil || got != want {
					t.Errorf("remote workspace branch = %q, %v, want %s", got, err, want)
				}
				if got := env.upstream(worktree); got != "origin/"+branch {
					t.Errorf("upstream after push = %q, want origin/%s", got, branch)
				}
			})
		}
	})
}

func TestWorkspacePushURLsAndGitCredentials(t *testing.T) {
	forEachUser(t, func(t *testing.T, env *env) {
		first := env.createRepoWithHistory("svc-auth", 1)
		second := env.createRepoWithHistory("svc-api", 1)
		helperKey := "credential.https://" + env.user.host + ".helper"
		env.git(env.home, "config", "--global", helperKey, "!gh auth git-credential")
		env.git(first, "config", "credential.helper", "cache")
		fetchURL := env.git(first, "config", "--get", "remote.origin.url")
		env.init()
		env.mustGW("create", "urls", "--branch", env.branch("urls"), "--repos", "svc-auth")
		env.mustGW("add-repo", "urls", "--repos", "svc-api")
		for _, repo := range []string{first, second} {
			want := "git@" + env.user.host + ":" + filepath.Base(repo) + "-origin.git"
			got, err := env.gitAllowFail(repo, "config", "--get", "remote.origin.pushurl")
			if err != nil || got != want {
				t.Errorf("%s push URL = %q, %v, want %q", repo, got, err, want)
			}
		}
		if got := env.git(first, "config", "--get", "remote.origin.url"); got != fetchURL {
			t.Errorf("fetch URL = %q, want %q", got, fetchURL)
		}
		if got := env.git(first, "config", "--local", "--get-all", "credential.helper"); got != "cache" {
			t.Errorf("existing helper changed to %q", got)
		}
		if got, err := env.gitAllowFail(second, "config", "--local", "--get-regexp", "^credential\\."); err == nil || got != "" {
			t.Errorf("Grove wrote credential configuration: %q, %v", got, err)
		}
		if got := env.git(second, "config", "--get-urlmatch", "credential.helper", "https://"+env.user.host+"/org/repo.git"); got != "!gh auth git-credential" {
			t.Errorf("host helper = %q", got)
		}
	})
}
