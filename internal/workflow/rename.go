package workflow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksenap/grove/internal/config"
	"github.com/nicksenap/grove/internal/contextrepo"
	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
)

// CheckRename requires clean context records and a free destination.
func (Hooks) CheckRename(ws models.Workspace, name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid workspace name %q", name)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := checkRecordsName(cfg, name); err != nil {
		return err
	}
	for _, repo := range ws.Repos {
		if !repo.Context {
			continue
		}
		if err := contextrepo.CheckRebase(repo); err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Join(repo.WorktreePath, "records", name)); err == nil {
			return fmt.Errorf("records already exist for %s", name)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// RenameRecords commits the records move without renaming the workspace branch.
func (Hooks) RenameRecords(ws models.Workspace, oldName string) error {
	for _, repo := range ws.Repos {
		if !repo.Context {
			continue
		}
		if err := renameRecords(ws, repo, oldName); err != nil {
			return err
		}
	}
	return nil
}

func renameRecords(ws models.Workspace, repo models.RepoWorktree, oldName string) (result error) {
	oldPath := filepath.Join("records", oldName)
	newPath := filepath.Join("records", ws.Name)
	data, err := os.ReadFile(filepath.Join(repo.WorktreePath, oldPath, "workspace.toml"))
	if err != nil {
		return err
	}
	var entry map[string]any
	if _, err := toml.Decode(string(data), &entry); err != nil {
		return err
	}
	entry["name"] = ws.Name
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(entry); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(repo.WorktreePath, oldPath), filepath.Join(repo.WorktreePath, newPath)); err != nil {
		return err
	}
	link := filepath.Join(ws.Path, "records")
	defer func() {
		if result != nil {
			result = errors.Join(result, gitops.RestorePaths(repo.WorktreePath, oldPath, newPath))
			_ = os.Remove(link)
			result = errors.Join(result, os.Symlink(filepath.Join(repo.RepoName, oldPath), link))
		}
	}()
	if err := os.WriteFile(filepath.Join(repo.WorktreePath, newPath, "workspace.toml"), encoded.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Remove(link); err != nil {
		return err
	}
	if err := os.Symlink(filepath.Join(repo.RepoName, newPath), link); err != nil {
		return err
	}
	return gitops.CommitPathsUnsigned(repo.WorktreePath, "docs(records): rename workspace records", oldPath, newPath)
}
