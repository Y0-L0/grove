package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/output"
	"github.com/nicksenap/grove/internal/state"
)

// IndexEntry combines committed records with authoritative active workspace state.
type IndexEntry struct {
	recordCreated string
	Sessions      []Session `json:"sessions"`
	output.Workspace
	Status       string   `json:"status"`
	Goal         string   `json:"goal"`
	Repositories []string `json:"repositories"`
}

// Index reads both configured contexts and the current active workspaces.
// Only with details does it read each goal and scan Claude's transcripts for sessions.
func Index(cfg *models.Config, details bool) ([]IndexEntry, error) {
	active, err := state.NewStore(cfg.StateDir).Load()
	if err != nil {
		return nil, err
	}
	base, err := baseEntries(cfg)
	if err != nil {
		return nil, err
	}
	byName := map[string]IndexEntry{}
	for _, stored := range base {
		if _, exists := byName[stored.Name]; exists {
			return nil, fmt.Errorf("workspace %s has records in multiple contexts", stored.Name)
		}
		if stored.Deleted.IsZero() {
			continue
		}
		entry, err := deletedIndexEntry(stored, cfg.WorkspaceDir, details)
		if err != nil {
			return nil, err
		}
		byName[stored.Name] = entry
	}
	for _, ws := range active {
		entry, err := activeIndexEntry(ws, details)
		if err != nil {
			return nil, err
		}
		byName[ws.Name] = entry
	}
	entries := make([]IndexEntry, 0, len(byName))
	for _, entry := range byName {
		entries = append(entries, entry)
	}
	if details && len(entries) > 0 {
		if err := addSessions(entries, earliestWorkspaceCreation(byName)); err != nil {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return indexLess(entries[i], entries[j]) })
	return entries, nil
}

// indexLess puts active workspaces before deleted ones,
// then the default type before the others in alphabetical order, then sorts by name.
func indexLess(a, b IndexEntry) bool {
	if deletedA, deletedB := a.Status == "deleted", b.Status == "deleted"; deletedA != deletedB {
		return deletedB
	}
	if a.Type != b.Type {
		if a.Type == DefaultType || b.Type == DefaultType {
			return a.Type == DefaultType
		}
		return a.Type < b.Type
	}
	return a.Name < b.Name
}

func addSessions(entries []IndexEntry, earliest time.Time) error {
	transcripts, err := readSessions(earliest)
	if err != nil {
		return err
	}
	for i, entry := range entries {
		created := entry.recordCreated
		if created == "" {
			created = entry.CreatedAt
		}
		entries[i].Sessions = workspaceSessions(entry.Path, created, transcripts)
	}
	return nil
}

func deletedIndexEntry(stored storedEntry, workspaceDir string, details bool) (IndexEntry, error) {
	entry := IndexEntry{
		Workspace: output.FromWorkspace(models.Workspace{Name: stored.Name, Type: stored.Type, Branch: stored.Branch, CreatedAt: stored.Created.Format("2006-01-02"), Path: filepath.Join(workspaceDir, stored.Type, stored.Name)}),
		Status:    "deleted", Repositories: stored.Repositories,
	}
	if !details {
		return entry, nil
	}
	baseRef, err := gitops.ResolveLocalBaseBranch(stored.source)
	if err != nil {
		return entry, err
	}
	goal, err := gitops.FileAt(stored.source, baseRef, filepath.ToSlash(filepath.Join("records", stored.Name, "TASK.md")))
	if err != nil {
		return entry, err
	}
	entry.Goal = goalSection(goal)
	return entry, nil
}

func activeIndexEntry(ws models.Workspace, details bool) (IndexEntry, error) {
	entry := IndexEntry{Workspace: output.FromWorkspace(ws), Status: "active", Repositories: []string{}}
	for _, repo := range ws.Repos {
		if !repo.Context {
			entry.Repositories = append(entry.Repositories, repo.RepoName)
			continue
		}
		if !details {
			continue
		}
		directory := filepath.Join(repo.WorktreePath, "records", ws.Name)
		metadata, err := os.ReadFile(filepath.Join(directory, "workspace.toml"))
		if err != nil {
			return entry, err
		}
		var record recordEntry
		if _, err := toml.Decode(string(metadata), &record); err != nil {
			return entry, err
		}
		entry.recordCreated = record.Created.Format("2006-01-02")
		goal, err := os.ReadFile(filepath.Join(directory, "TASK.md"))
		if err != nil {
			return entry, err
		}
		entry.Goal = goalSection(string(goal))
	}
	return entry, nil
}

func goalSection(markdown string) string {
	var lines []string
	inGoal := false
	for _, line := range strings.Split(markdown, "\n") {
		if strings.TrimSpace(line) == "## Goal" {
			inGoal = true
			continue
		}
		if inGoal && (strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ")) {
			break
		}
		if inGoal {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
