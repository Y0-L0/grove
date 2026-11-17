// Package contextrepo defines the policies selected by a context worktree's flag.
package contextrepo

import (
	"fmt"

	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
)

// BaseBranch selects the local base for context worktrees.
func BaseBranch(repo models.RepoWorktree) (string, error) {
	if repo.Context {
		return gitops.ResolveLocalBaseBranch(repo.SourceRepo)
	}
	return gitops.ResolveBaseBranch(repo.SourceRepo)
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
