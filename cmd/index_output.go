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

// writeIndex renders the index with one line per workspace. The table and TSV
// formats print name, type, status, the number of repositories besides
// the context repository and branch; the wide format adds their names,
// the session count and the first line of the goal.
// The sessions themselves appear only in JSON and in writeIndexEntry.
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
	case output.Table, output.Wide, output.TSV:
		rows := indexRows(entries, format == output.Wide)
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

func indexRows(entries []workflow.IndexEntry, wide bool) [][]string {
	header := []string{"Name", "Type", "Status", "Repos", "Branch"}
	if wide {
		header = append(header, "Repositories", "Sessions", "Goal")
	}
	rows := [][]string{header}
	for _, entry := range entries {
		row := []string{entry.Name, entry.Type, entry.Status, strconv.Itoa(len(entry.Repositories)), entry.Branch}
		if wide {
			row = append(row, strings.Join(entry.Repositories, ","), strconv.Itoa(len(entry.Sessions)), firstLine(entry.Goal))
		}
		rows = append(rows, row)
	}
	return rows
}

// writeIndexEntry renders one workspace with every column: its index line,
// then in the table formats each session with the command that resumes it.
func writeIndexEntry(w io.Writer, entry workflow.IndexEntry, format output.Format) error {
	entries := []workflow.IndexEntry{entry}
	switch format {
	case output.TSV:
		return output.WriteTSV(w, indexRows(entries, true))
	case output.Table, output.Wide:
		if err := writeIndex(w, entries, output.Wide); err != nil {
			return err
		}
	default:
		return writeIndex(w, entries, format)
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
