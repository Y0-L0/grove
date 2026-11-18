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
	"github.com/nicksenap/grove/internal/workspace"
)

type recordEntry struct {
	Name         string    `toml:"name"`
	Branch       string    `toml:"branch"`
	Type         string    `toml:"type"`
	Goal         string    `toml:"goal"`
	Created      time.Time `toml:"created"`
	Deleted      time.Time `toml:"deleted"`
	Repositories []string  `toml:"repositories"`
}

type storedEntry struct {
	recordEntry
	source string
	path   string
}

func baseEntries(cfg *models.Config) ([]storedEntry, error) {
	entries := []storedEntry{}
	seen := map[string]bool{}
	for _, kind := range cfg.Types {
		if kind.Context == "" {
			continue
		}
		source, err := contextPath(kind.Context)
		if err != nil {
			return nil, err
		}
		if seen[source] {
			continue
		}
		seen[source] = true
		base, err := gitops.ResolveLocalBaseBranch(source)
		if err != nil {
			return nil, err
		}
		paths, err := gitops.TreePaths(source, base, "records")
		if err != nil {
			return nil, err
		}
		for _, path := range strings.Split(paths, "\n") {
			parts := strings.Split(path, "/")
			if len(parts) != 3 || parts[2] != "workspace.toml" {
				continue
			}
			data, err := gitops.FileAt(source, base, path)
			if err != nil {
				return nil, err
			}
			var entry recordEntry
			if _, err := toml.Decode(data, &entry); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			if entry.Name != parts[1] || entry.Branch == "" {
				return nil, fmt.Errorf("invalid workspace entry %s", path)
			}
			if err := ValidateType(entry.Type); err != nil {
				return nil, err
			}
			configured, err := contextPath(cfg.Types[entry.Type].Context)
			if err != nil || configured != source {
				return nil, fmt.Errorf("entry %s has an unconfigured context repository", path)
			}
			entries = append(entries, storedEntry{entry, source, path})
		}
	}
	return entries, nil
}

// Reopen recreates a deleted workspace with its context worktree only.
// A failed metadata commit leaves the workspace tracked so another call can retry.
func Reopen(name string, cfg *models.Config) ([]string, error) {
	entries, err := baseEntries(cfg)
	if err != nil {
		return nil, err
	}
	found, err := findDeletedEntry(entries, name)
	if err != nil {
		return nil, err
	}
	if err := gitops.CheckBranchName(found.Branch); err != nil {
		return nil, err
	}
	store := state.NewStore(cfg.StateDir)
	ws, err := store.GetWorkspace(name)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		if err := workspace.NewService().CreateWithOpts(name, workspace.CreateOpts{Type: found.Type, Branch: found.Branch, Cfg: cfg}); err != nil {
			return nil, err
		}
		ws, err = store.GetWorkspace(name)
		if err != nil {
			return nil, err
		}
	}
	if ws == nil || ws.Type != found.Type || ws.Branch != found.Branch {
		return nil, errors.New("active workspace does not match deleted records")
	}
	var repo models.RepoWorktree
	if err := store.WithLock(func() error {
		current, err := store.GetWorkspace(name)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("workspace %s disappeared", name)
		}
		repo, err = attachContext(current, found.source, store)
		*ws = *current
		return err
	}); err != nil {
		return nil, err
	}
	if err := rebaseRulesContext(repo); err != nil {
		return nil, err
	}
	if err := reopenEntry(*ws, repo); err != nil {
		return nil, err
	}
	if err := GenerateRules(*ws, cfg); err != nil {
		return nil, err
	}
	return found.Repositories, nil
}

func findDeletedEntry(entries []storedEntry, name string) (*storedEntry, error) {
	var found *storedEntry
	for i := range entries {
		if entries[i].Name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("workspace %s has records in multiple context repositories", name)
		}
		found = &entries[i]
	}
	if found == nil {
		return nil, fmt.Errorf("workspace %s has no records", name)
	}
	if found.Deleted.IsZero() {
		return nil, fmt.Errorf("workspace %s is not deleted", name)
	}
	return found, nil
}

func reopenEntry(ws models.Workspace, repo models.RepoWorktree) error {
	path := filepath.Join("records", ws.Name, "workspace.toml")
	data, err := os.ReadFile(filepath.Join(repo.WorktreePath, path))
	if err != nil {
		return err
	}
	var entry map[string]any
	if _, err := toml.Decode(string(data), &entry); err != nil {
		return err
	}
	if _, deleted := entry["deleted"]; !deleted {
		return fmt.Errorf("workspace %s is already active", ws.Name)
	}
	link := filepath.Join(ws.Path, "records")
	target := filepath.Join(repo.RepoName, "records", ws.Name)
	if current, err := os.Readlink(link); os.IsNotExist(err) {
		if err := os.Symlink(target, link); err != nil {
			return err
		}
	} else if err != nil || current != target {
		return errors.New("workspace records symlink has an unexpected target")
	}
	delete(entry, "deleted")
	delete(entry, "repositories")
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(entry); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo.WorktreePath, path), encoded.Bytes(), 0o644); err != nil {
		return err
	}
	if err := gitops.CommitPathsUnsigned(repo.WorktreePath, "docs(records): reopen workspace", path); err != nil {
		return errors.Join(err, gitops.RestorePaths(repo.WorktreePath, path))
	}
	return nil
}
