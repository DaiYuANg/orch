package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"

	"github.com/arcgolabs/collectionx/list"

	"github.com/daiyuang/orch/pkg/oopsx"
)

var (
	viewBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	viewHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1)
	viewCellStyle    = lipgloss.NewStyle().Padding(0, 1)
	viewOddCellStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Padding(0, 1)
	viewMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	viewOKStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	viewWarnStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	viewErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	viewInfoStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	viewKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
)

func writeInfoLine(label string, fields ...string) error {
	parts := list.NewList(viewInfoStyle.Render(strings.ToUpper(strings.TrimSpace(label))))
	parts.Add(fields...)
	return writeLine(parts.Join(" "))
}

func viewField(key, value string) string {
	return viewMutedStyle.Render(strings.TrimSpace(key)+"=") + strings.TrimSpace(value)
}

func writeTable(headers *list.List[string], rows *list.Grid[string]) error {
	if rows.RowCount() == 0 {
		return writeLine(viewMutedStyle.Render("No resources found."))
	}
	t := lgtable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(viewBorderStyle).
		BorderRow(false).
		Headers(headers.Values()...).
		Rows(rows.Values()...).
		StyleFunc(tableStyle)
	return writeLine(t.Render())
}

func writeKVTable(rows *list.Grid[string]) error {
	t := lgtable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(viewBorderStyle).
		BorderColumn(true).
		BorderRow(false).
		Headers("PROPERTY", "VALUE").
		Rows(rows.Values()...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == lgtable.HeaderRow {
				return viewHeaderStyle
			}
			if col == 0 {
				return viewKeyStyle
			}
			return viewCellStyle
		})
	return writeLine(t.Render())
}

func tableStyle(row, _ int) lipgloss.Style {
	if row == lgtable.HeaderRow {
		return viewHeaderStyle
	}
	if row%2 == 1 {
		return viewOddCellStyle
	}
	return viewCellStyle
}

func statusBadge(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	label := strings.ToUpper(nonEmpty(normalized))
	switch normalized {
	case "ok", "ready", "running", "accepted":
		return viewOKStyle.Render(label)
	case "assigned", "pending", "starting":
		return viewWarnStyle.Render(label)
	case "failed", "error", "unhealthy":
		return viewErrorStyle.Render(label)
	default:
		return viewInfoStyle.Render(label)
	}
}

func formatPercent(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64) + "%"
}

func formatBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return strconv.FormatUint(v, 10) + " B"
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := list.NewList("KiB", "MiB", "GiB", "TiB", "PiB", "EiB")
	value := float64(v) / float64(div)
	unitLabel, _ := units.Get(exp)
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + unitLabel
}

func writeLine(s string) error {
	if _, err := fmt.Fprintln(os.Stdout, s); err != nil {
		return oopsx.B("cli").Wrapf(err, "write terminal output")
	}
	return nil
}
