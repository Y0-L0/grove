package workflow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		if err := workspace.CheckDelete(ws); err != nil {
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

func mergeRecords(ws models.Workspace, repo models.RepoWorktree, force bool) error {
	base, err := contextrepo.CheckedOutBase(repo)
	if err != nil {
		return err
	}
	if err := prepareContextCheckout(repo, base, force); err != nil {
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

func prepareContextCheckout(repo models.RepoWorktree, base string, force bool) error {
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
		// A retry must retain commits made on an unexpected branch or detached HEAD.
		if err := gitops.CheckAncestor(repo.WorktreePath, "HEAD", "refs/heads/"+repo.Branch); err != nil {
			if baseErr := gitops.CheckAncestor(repo.WorktreePath, "HEAD", base); baseErr != nil {
				return fmt.Errorf("context checkout has committed records outside %s and %s; merge its HEAD into the workspace branch before retrying: %w", repo.Branch, base, errors.Join(err, baseErr))
			}
		}
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
