package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nicksenap/grove/internal/contextrepo"
	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
)

func checkRemoval(ws models.Workspace, opts RemoveOptions) error {
	if err := checkStoredBranches(ws); err != nil {
		return err
	}
	if !opts.RecordsMerged {
		return preflightRemovals(ws.Repos, opts.Force)
	}
	var problems []error
	if !opts.Force {
		problems = append(problems, CheckDelete(ws))
	}
	// Hooks may commit more records after the merge, even on another checkout branch.
	// Force permits discarding code and dirty files, but committed records must survive.
	for _, repo := range ws.Repos {
		if repo.Context {
			if err := checkRecordsMerged(repo); err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", repo.RepoName, err))
			}
		}
	}
	return errors.Join(problems...)
}

func checkRecordsMerged(repo models.RepoWorktree) error {
	base, err := contextrepo.BaseBranch(repo)
	if err != nil {
		return err
	}
	if err := gitops.CheckAncestor(repo.SourceRepo, "refs/heads/"+repo.Branch, base); err != nil {
		return fmt.Errorf("context records on %s have not been merged into %s; retry delete after disabling the mutating hook: %w", repo.Branch, base, err)
	}
	if _, err := os.Stat(repo.WorktreePath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := gitops.CheckAncestor(repo.WorktreePath, "HEAD", base); err != nil {
		return fmt.Errorf("context checkout has committed records not merged into %s: %w", base, err)
	}
	return nil
}

func checkStoredBranches(ws models.Workspace) error {
	var problems []error
	for _, repo := range ws.Repos {
		if err := repo.CheckStoredBranch(ws.Name); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", repo.RepoName, err))
		}
	}
	return errors.Join(problems...)
}

// CheckDelete collects every workspace deletion problem without changing Git or state.
func CheckDelete(ws models.Workspace) error {
	if err := checkStoredBranches(ws); err != nil {
		return err
	}
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
		if err := preflightRemoval(repo); err != nil {
			problems = append(problems, err)
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
		if _, err := contextrepo.CheckedOutBase(repo); err != nil {
			problems = append(problems, err)
		}
	}
	return problems
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
			if canonicalPath(path) == canonicalPath(repo.WorktreePath) {
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
