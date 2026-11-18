package workflow

import (
	"fmt"
	"io"
	"strings"

	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
)

type signingRepo struct {
	repo models.RepoWorktree
	base string
}

// Sign shows, signs and verifies every code branch before allowing any push.
// Context commits stay unsigned and are never pushed.
func Sign(ws models.Workspace, out io.Writer) error {
	repos := []signingRepo{}
	for _, repo := range ws.Repos {
		if repo.Context {
			continue
		}
		fmt.Fprintf(out, "%s: %s\n", repo.RepoName, repo.Branch)
		branch, err := gitops.CurrentBranch(repo.WorktreePath)
		if err != nil {
			return err
		}
		if branch != repo.Branch {
			return fmt.Errorf("%s is on %q, expected %q", repo.RepoName, branch, repo.Branch)
		}
		status, err := gitops.RepoStatus(repo.WorktreePath)
		if err != nil {
			return err
		}
		if status != "" {
			return fmt.Errorf("%s is dirty; commit or stash before signing", repo.RepoName)
		}
		base, err := gitops.ResolveBaseBranch(repo.SourceRepo)
		if err != nil {
			return err
		}
		if repo.Branch == strings.TrimPrefix(base, "origin/") {
			return fmt.Errorf("refusing to sign and push base branch %s", repo.Branch)
		}
		repos = append(repos, signingRepo{repo, base})
	}
	for _, selected := range repos {
		statuses, err := gitops.CommitSignatures(selected.repo.WorktreePath, selected.base, selected.repo.Branch)
		if err != nil {
			return err
		}
		if strings.Contains(statuses, "N") {
			if err := gitops.SignCommits(selected.repo.WorktreePath, selected.base); err != nil {
				return fmt.Errorf("signing %s: %w", selected.repo.RepoName, err)
			}
		}
	}
	for _, selected := range repos {
		statuses, err := gitops.CommitSignatures(selected.repo.WorktreePath, selected.base, selected.repo.Branch)
		if err != nil {
			return err
		}
		for _, status := range strings.Fields(statuses) {
			if status != "G" && status != "U" {
				return fmt.Errorf("%s still has unsigned or unverifiable commits; nothing pushed", selected.repo.RepoName)
			}
		}
	}
	for _, selected := range repos {
		if err := gitops.PushWorkspaceBranch(selected.repo.WorktreePath, selected.repo.Branch); err != nil {
			return fmt.Errorf("pushing %s: %w", selected.repo.RepoName, err)
		}
	}
	return nil
}
