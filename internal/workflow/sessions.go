package workflow

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session describes a Claude transcript and the directory needed to resume it.
type Session struct {
	ID             string `json:"id"`
	FirstTimestamp string `json:"first_timestamp"`
	LastTimestamp  string `json:"last_timestamp"`
	AITitle        string `json:"ai_title"`
	Cwd            string `json:"cwd"`
	ResumeCommand  string `json:"resume_command"`
}

type transcriptSession struct {
	Session
	directories []string
	modified    time.Time
	first, last time.Time
}

func readSessions(earliest time.Time) ([]transcriptSession, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sessions := []transcriptSession{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.ModTime().Before(earliest) {
			continue
		}
		session, err := readTranscript(path)
		if err != nil {
			return nil, fmt.Errorf("reading Claude transcript %s: %w", path, err)
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func readTranscript(path string) (transcriptSession, error) {
	f, err := os.Open(path)
	if err != nil {
		return transcriptSession{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return transcriptSession{}, err
	}
	session := transcriptSession{Session: Session{ID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}, modified: info.ModTime()}
	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadBytes(10)
		var event struct {
			SessionID string `json:"sessionId"`
			Cwd       string `json:"cwd"`
			Timestamp string `json:"timestamp"`
			AITitle   string `json:"aiTitle"`
		}
		if json.Unmarshal(line, &event) == nil {
			if event.SessionID != "" {
				session.ID = event.SessionID
			}
			if event.AITitle != "" {
				session.AITitle = event.AITitle
			}
			if event.Cwd != "" {
				if session.Cwd == "" {
					session.Cwd = event.Cwd
				}
				session.directories = append(session.directories, event.Cwd)
			}
			if stamp, err := time.Parse(time.RFC3339Nano, event.Timestamp); err == nil {
				if session.first.IsZero() || stamp.Before(session.first) {
					session.FirstTimestamp = event.Timestamp
					session.first = stamp
				}
				if session.last.IsZero() || stamp.After(session.last) {
					session.LastTimestamp = event.Timestamp
					session.last = stamp
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return transcriptSession{}, readErr
		}
	}
	session.ResumeCommand = "cd " + shellQuote(session.Cwd) + " && \\\nclaude --resume " + shellQuote(session.ID)
	return session, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func workspaceSessions(path, created string, transcripts []transcriptSession) []Session {
	sessions := []Session{}
	date, _ := time.Parse("2006-01-02", strings.SplitN(created, "T", 2)[0])
	seen := map[string]bool{}
	for _, transcript := range transcripts {
		if transcript.modified.Before(date) || seen[transcript.ID] {
			continue
		}
		for _, cwd := range transcript.directories {
			rel, err := filepath.Rel(filepath.Clean(path), filepath.Clean(cwd))
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			sessions = append(sessions, transcript.Session)
			seen[transcript.ID] = true
			break
		}
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return sessions
}

func earliestWorkspaceCreation(entries map[string]IndexEntry) time.Time {
	var earliest time.Time
	for _, entry := range entries {
		created := entry.recordCreated
		if created == "" {
			created = entry.CreatedAt
		}
		date, err := time.Parse("2006-01-02", strings.SplitN(created, "T", 2)[0])
		if err != nil {
			return time.Time{}
		}
		if earliest.IsZero() || date.Before(earliest) {
			earliest = date
		}
	}
	return earliest
}
