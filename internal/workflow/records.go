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
	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/state"
)

func createRecords(ws models.Workspace, cfg *models.Config) error {
	if cfg == nil || ws.Type == "" || cfg.Types[ws.Type].Context == "" {
		return nil
	}
	source, err := contextPath(cfg.Types[ws.Type].Context)
	if err != nil {
		return err
	}
	if filepath.Base(source) == "records" {
		return errors.New("context repository name records is reserved for the records symlink")
	}
	if _, err := os.Lstat(filepath.Join(ws.Path, "records")); err == nil {
		return errors.New("workspace records path already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	store := state.NewStore(cfg.StateDir)
	var repo models.RepoWorktree
	if err := store.WithLock(func() error {
		current, err := store.GetWorkspace(ws.Name)
		if err != nil {
			return err
		}
		if current == nil || current.Path != ws.Path || current.CreatedAt != ws.CreatedAt || current.Branch != ws.Branch {
			return fmt.Errorf("workspace %s changed before context creation", ws.Name)
		}
		repo, err = attachContext(current, source, store)
		return err
	}); err != nil {
		return fmt.Errorf("attaching context repository: %w", err)
	}
	// Git commit hooks may invoke gw, so release the state lock before committing.
	if err := initializeRecords(ws, repo); err != nil {
		return fmt.Errorf("initializing records for %s; the context worktree remains tracked: %w", ws.Name, err)
	}
	return nil
}

func contextPath(source string) (string, error) {
	if strings.HasPrefix(source, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		source = filepath.Join(home, source[2:])
	}
	return filepath.Abs(source)
}

func attachContext(ws *models.Workspace, source string, store *state.Store) (models.RepoWorktree, error) {
	name := filepath.Base(source)
	path := filepath.Join(ws.Path, name)
	if existing := ws.FindRepo(name); existing != nil {
		if filepath.Clean(existing.SourceRepo) != source || existing.WorktreePath != path || existing.Branch != ws.Branch {
			return models.RepoWorktree{}, fmt.Errorf("context repository name %s is already in the workspace for a different repository", name)
		}
		existing.Context = true
		if err := store.UpdateWorkspace(*ws); err != nil {
			return models.RepoWorktree{}, err
		}
		_, err := gitops.ResolveLocalBaseBranch(source)
		return *existing, err
	}
	repo := models.RepoWorktree{
		RepoName: name, SourceRepo: source, WorktreePath: path, Branch: ws.Branch, Context: true,
	}
	created, err := provisionContext(repo)
	if err != nil {
		return models.RepoWorktree{}, err
	}
	ws.Repos = append(ws.Repos, repo)
	if err := store.UpdateWorkspace(*ws); err != nil {
		cleanup := gitops.WorktreeRemove(source, path, false)
		if created && cleanup == nil {
			cleanup = gitops.DeleteBranch(source, ws.Branch, false)
		}
		return models.RepoWorktree{}, errors.Join(err, cleanup)
	}
	return repo, nil
}

func provisionContext(repo models.RepoWorktree) (bool, error) {
	base, err := gitops.ResolveLocalBaseBranch(repo.SourceRepo)
	if err != nil {
		return false, err
	}
	created := !gitops.BranchExists(repo.SourceRepo, repo.Branch)
	if created {
		if err := gitops.CreateBranch(repo.SourceRepo, repo.Branch, base); err != nil {
			return false, err
		}
	}
	if err := gitops.WorktreeAdd(repo.SourceRepo, repo.WorktreePath, repo.Branch); err != nil {
		if created {
			return false, errors.Join(err, gitops.DeleteBranch(repo.SourceRepo, repo.Branch, false))
		}
		return false, err
	}
	return created, nil
}

func initializeRecords(ws models.Workspace, repo models.RepoWorktree) error {
	records := filepath.Join(repo.WorktreePath, "records", ws.Name)
	if _, err := os.Lstat(records); err == nil {
		return fmt.Errorf("records already exist for %s; reopen requires rewrite slice 9", ws.Name)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(records, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(records, "TASK.md"), []byte("# "+ws.Name+"\n\n## Goal\n\n## State\n"), 0o644); err != nil {
		return err
	}
	if err := writeEntry(records, ws); err != nil {
		return err
	}
	if err := os.Symlink(filepath.Join(repo.RepoName, "records", ws.Name), filepath.Join(ws.Path, "records")); err != nil {
		return err
	}
	return gitops.CommitPathsUnsigned(repo.WorktreePath, "docs(records): create workspace records",
		filepath.Join("records", ws.Name, "TASK.md"), filepath.Join("records", ws.Name, "workspace.toml"))
}

func writeEntry(records string, ws models.Workspace) error {
	created, err := time.Parse("2006-01-02T15:04:05.000000", ws.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid workspace creation time: %w", err)
	}
	entry := struct {
		Name   string `toml:"name"`
		Branch string `toml:"branch"`
		Type   string `toml:"type"`
		Goal   string `toml:"goal"`
	}{ws.Name, ws.Branch, ws.Type, "TASK.md#goal"}
	var data bytes.Buffer
	if err := toml.NewEncoder(&data).Encode(entry); err != nil {
		return err
	}
	fmt.Fprintf(&data, "created = %s\n", created.Format("2006-01-02"))
	return os.WriteFile(filepath.Join(records, "workspace.toml"), data.Bytes(), 0o644)
}
