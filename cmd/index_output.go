package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nicksenap/grove/internal/console"
	"github.com/nicksenap/grove/internal/output"
	"github.com/nicksenap/grove/internal/workflow"
)

// writeIndex renders the index. The table and TSV formats print one line per
// workspace; the sessions themselves appear only in JSON and in writeIndexEntry.
func writeIndex(w io.Writer, entries []workflow.IndexEntry, format output.Format) error {
	switch format {
	case output.JSON:
		return output.WriteJSON(w, entries)
	case output.JSONLines:
		return output.WriteJSONLines(w, entries)
	case output.Name, output.Path:
		for _, entry := range entries {
			value := entry.Name
			if format == output.Path {
				value = entry.Path
			}
			if _, err := fmt.Fprintln(w, value); err != nil {
				return err
			}
		}
		return nil
	case output.Table, output.TSV:
		rows := [][]string{{"Name", "Type", "Status", "Branch", "Repositories", "Sessions", "Goal"}}
		for _, entry := range entries {
			rows = append(rows, []string{entry.Name, entry.Type, entry.Status, entry.Branch, strings.Join(entry.Repositories, ","), strconv.Itoa(len(entry.Sessions)), firstLine(entry.Goal)})
		}
		if format == output.TSV {
			return output.WriteTSV(w, rows)
		}
		table := console.NewTable(w, rows[0])
		for _, row := range rows[1:] {
			table.AddRow(row)
		}
		table.Render()
		return nil
	default:
		return fmt.Errorf("unsupported index format %q", format)
	}
}

// writeIndexEntry renders one workspace: its index line, then in the table
// format each session with the command that resumes it.
func writeIndexEntry(w io.Writer, entry workflow.IndexEntry, format output.Format) error {
	if err := writeIndex(w, []workflow.IndexEntry{entry}, format); err != nil || format != output.Table {
		return err
	}
	for _, session := range entry.Sessions {
		if _, err := fmt.Fprintf(w, "\n%s %s %s %s\n%s\n", session.ID, session.FirstTimestamp, session.LastTimestamp, session.AITitle, session.ResumeCommand); err != nil {
			return err
		}
	}
	return nil
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return strings.TrimSpace(line)
}
