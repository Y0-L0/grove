package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/nicksenap/grove/internal/console"
	"github.com/nicksenap/grove/internal/output"
	"github.com/nicksenap/grove/internal/workflow"
)

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
		rows := [][]string{{"Name", "Type", "Status", "Branch", "Repositories", "Goal"}}
		for _, entry := range entries {
			rows = append(rows, []string{entry.Name, entry.Type, entry.Status, entry.Branch, strings.Join(entry.Repositories, ","), entry.Goal})
		}
		if format == output.TSV {
			return output.WriteTSV(w, rows)
		}
		table := console.NewTable(w, rows[0])
		for _, row := range rows[1:] {
			table.AddRow(row)
		}
		table.Render()
		for _, entry := range entries {
			for _, session := range entry.Sessions {
				if _, err := fmt.Fprintf(w, "%s: %s %s %s %s\n%s\n", entry.Name, session.ID, session.FirstTimestamp, session.LastTimestamp, session.AITitle, session.ResumeCommand); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported index format %q", format)
	}
}
