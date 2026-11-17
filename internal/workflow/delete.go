package workflow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksenap/grove/internal/contextrepo"
	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/workspace"
)

// PrepareDelete checks the entire workspace before committing or merging records.
// This policy also applies when optional lifecycle hooks are disabled.
func (Hooks) PrepareDelete(ws models.Workspace, opts workspace.RemoveOptions) error {
	if !opts.Force {
		if err := checkDelete(ws); err != nil {
			return err
		}
	}
	for _, repo := range ws.Repos {
		if repo.Context {
			if err := mergeRecords(ws, repo, opts.Force); err != nil {
				return fmt.Errorf("%s: preserving context records: %w", repo.RepoName, err)
			}
		}
	}
	return nil
}

func checkDelete(ws models.Workspace) error {
	var problems []error
	for _, repo := range ws.Repos {
		for _, err := range checkDeleteRepo(repo) {
			problems = append(problems, fmt.Errorf("%s: %w", repo.RepoName, err))
		}
	}
	problems = append(problems, checkTemporaryWorktrees(ws)...)
	return errors.Join(problems...)
}

func checkDeleteRepo(repo models.RepoWorktree) []error {
	var problems []error
	if _, err := os.Stat(repo.WorktreePath); err == nil {
		problems = append(problems, checkRegistration(repo)...)
		status, err := gitops.RepoStatus(repo.WorktreePath)
		if err != nil {
			problems = append(problems, err)
		} else if status != "" {
			problems = append(problems, fmt.Errorf("dirty worktree at %s:\n%s", repo.WorktreePath, status))
		}
	} else if !os.IsNotExist(err) {
		problems = append(problems, err)
	}
	if !repo.Context {
		commits, err := gitops.UnpushedCommits(repo.SourceRepo, repo.Branch)
		if err != nil {
			problems = append(problems, err)
		} else if commits != "" {
			problems = append(problems, fmt.Errorf("unpushed commits on %s:\n%s", repo.Branch, commits))
		}
	} else {
		if _, err := checkedOutBase(repo); err != nil {
			problems = append(problems, err)
		}
	}
	return problems
}

func checkRegistration(repo models.RepoWorktree) []error {
	entries, err := gitops.WorktreeList(repo.SourceRepo)
	if err != nil {
		return []error{err}
	}
	for _, entry := range entries {
		if samePath(entry.Path, repo.WorktreePath) {
			if entry.Branch != repo.Branch {
				return []error{fmt.Errorf("unexpected branch %q at %s, expected %q", entry.Branch, repo.WorktreePath, repo.Branch)}
			}
			return nil
		}
	}
	return []error{fmt.Errorf("worktree path is not registered: %s", repo.WorktreePath)}
}

func samePath(a, b string) bool {
	canonical := func(path string) string {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		absolute, err := filepath.Abs(path)
		if err == nil {
			path = absolute
		}
		return filepath.Clean(path)
	}
	return canonical(a) == canonical(b)
}

func checkTemporaryWorktrees(ws models.Workspace) []error {
	entries, err := os.ReadDir(ws.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []error{err}
	}
	var problems []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(ws.Path, entry.Name())
		tracked := false
		for _, repo := range ws.Repos {
			if samePath(path, repo.WorktreePath) {
				tracked = true
			}
		}
		if tracked {
			continue
		}
		info, err := os.Lstat(filepath.Join(path, ".git"))
		if err == nil && info.Mode().IsRegular() {
			problems = append(problems, fmt.Errorf("temporary worktree %s; remove it with:\ngit -C %s \\\n  worktree remove \\\n  %s", path, shellQuote(path), shellQuote(path)))
		} else if err != nil && !os.IsNotExist(err) {
			problems = append(problems, err)
		}
	}
	return problems
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func checkedOutBase(repo models.RepoWorktree) (string, error) {
	base, err := contextrepo.BaseBranch(repo)
	if err != nil {
		return "", err
	}
	branch, err := gitops.CurrentBranch(repo.SourceRepo)
	if err != nil {
		return "", err
	}
	if "refs/heads/"+branch != base {
		return "", fmt.Errorf("context clone must have base branch %q checked out, found %q", base, branch)
	}
	return base, nil
}

func mergeRecords(ws models.Workspace, repo models.RepoWorktree, force bool) error {
	base, err := checkedOutBase(repo)
	if err != nil {
		return err
	}
	if err := prepareContextCheckout(repo, force); err != nil {
		return err
	}
	if err := recordDeletion(ws, repo); err != nil {
		return err
	}
	if err := gitops.RebaseUnsigned(repo.WorktreePath, base); err != nil {
		return errors.Join(err, gitops.RebaseAbort(repo.WorktreePath))
	}
	return gitops.FastForward(repo.SourceRepo, repo.Branch)
}

func prepareContextCheckout(repo models.RepoWorktree, force bool) error {
	if _, err := os.Stat(repo.WorktreePath); os.IsNotExist(err) {
		// Missing checkouts still have committed records on their stored branch.
		if err := gitops.WorktreePrune(repo.SourceRepo); err != nil {
			return err
		}
		if err := gitops.WorktreeAdd(repo.SourceRepo, repo.WorktreePath, repo.Branch); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if force {
		if err := gitops.DiscardWorktreeChanges(repo.WorktreePath); err != nil {
			return err
		}
		if err := gitops.Switch(repo.WorktreePath, repo.Branch, false); err != nil {
			return err
		}
	}
	return nil
}

func recordDeletion(ws models.Workspace, repo models.RepoWorktree) error {
	path := filepath.Join("records", ws.Name, "workspace.toml")
	data, err := os.ReadFile(filepath.Join(repo.WorktreePath, path))
	if err != nil {
		return err
	}
	var entry map[string]any
	if _, err := toml.Decode(string(data), &entry); err != nil {
		return err
	}
	if _, exists := entry["deleted"]; !exists {
		// Decode the date to retain TOML's local-date type when encoding the entry.
		var date map[string]any
		if _, err := toml.Decode("deleted = "+time.Now().Format("2006-01-02"), &date); err != nil {
			return err
		}
		entry["deleted"] = date["deleted"]
	}
	repos := make([]string, 0, len(ws.Repos))
	for _, r := range ws.Repos {
		if !r.Context {
			repos = append(repos, r.RepoName)
		}
	}
	entry["repositories"] = repos
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(entry); err != nil {
		return err
	}
	if !bytes.Equal(data, encoded.Bytes()) {
		if err := os.WriteFile(filepath.Join(repo.WorktreePath, path), encoded.Bytes(), 0o644); err != nil {
			return err
		}
		if err := gitops.CommitPathsUnsigned(repo.WorktreePath, "docs(records): retain deleted workspace history", path); err != nil {
			return errors.Join(err, gitops.RestorePaths(repo.WorktreePath, path))
		}
	}
	return nil
}
