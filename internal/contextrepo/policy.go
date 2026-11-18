// Package contextrepo defines the policies selected by a context worktree's flag.
package contextrepo

import (
	"errors"
	"fmt"

	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
)

// CheckRebase requires a clean context checkout on its recorded branch.
func CheckRebase(repo models.RepoWorktree) error {
	branch, err := gitops.CurrentBranch(repo.WorktreePath)
	if err != nil {
		return err
	}
	if branch != repo.Branch {
		return fmt.Errorf("context worktree is on %q, expected %q", branch, repo.Branch)
	}
	status, err := gitops.RepoStatus(repo.WorktreePath)
	if err != nil {
		return err
	}
	if status != "" {
		return errors.New("context worktree is dirty; commit or stash changes before rebasing")
	}
	return nil
}

// BaseBranch selects the local base for context worktrees.
func BaseBranch(repo models.RepoWorktree) (string, error) {
	if repo.Context {
		return gitops.ResolveLocalBaseBranch(repo.SourceRepo)
	}
	return gitops.ResolveBaseBranch(repo.SourceRepo)
}

// CheckedOutBase requires the context clone to have its local base checked out.
func CheckedOutBase(repo models.RepoWorktree) (string, error) {
	base, err := BaseBranch(repo)
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

// CheckDelete requires the operation policy to preserve records before removal.
func CheckDelete(repos []models.RepoWorktree, merged bool) error {
	for _, repo := range repos {
		if repo.Context && !merged {
			return fmt.Errorf("cannot delete context repository %s before records merging", repo.RepoName)
		}
	}
	return nil
}

// CheckRemove keeps a workspace's context repository attached.
func CheckRemove(repos []models.RepoWorktree) error {
	for _, repo := range repos {
		if repo.Context {
			return fmt.Errorf("cannot remove context repository %s", repo.RepoName)
		}
	}
	return nil
}
