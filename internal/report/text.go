package report

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/reporter"
	"github.com/space-code/linkctl/pkg/iostreams"
)

// WriteText renders r as the same colourised, human-readable style used
// elsewhere in linkctl (internal/reporter).
func WriteText(w io.Writer, cs *iostreams.ColorScheme, r *Report) {
	fmt.Fprintf(w, "\n%s %s\n", cs.Bold("●"), cs.Bold(r.Target))

	for _, section := range r.Sections {
		fmt.Fprintf(w, "\n%s\n", cs.Bold(section.Name))
		writeKV(w, cs, section.KV)
		writeChecks(w, cs, section.Checks)
	}

	summary := r.Summary()
	fmt.Fprintf(
		w, "\nSummary: %s   %s   %s\n\n",
		cs.Green(fmt.Sprintf("%d passed", summary.Passed)),
		cs.Red(fmt.Sprintf("%d failed", summary.Failed)),
		cs.Yellow(fmt.Sprintf("%d warnings", summary.Warnings)),
	)
}

func writeKV(w io.Writer, cs *iostreams.ColorScheme, rows [][2]string) {
	if len(rows) == 0 {
		return
	}
	maxKey := 0
	for _, row := range rows {
		if n := utf8.RuneCountInString(row[0]); n > maxKey {
			maxKey = n
		}
	}
	for _, row := range rows {
		pad := maxKey - utf8.RuneCountInString(row[0])
		fmt.Fprintf(w, "  %s%s  %s\n", cs.Muted(row[0]+":"), strings.Repeat(" ", pad), row[1])
	}
}

func writeChecks(w io.Writer, cs *iostreams.ColorScheme, checks []models.ValidationResult) {
	if len(checks) == 0 {
		return
	}
	maxWidth := 0
	for _, c := range checks {
		if n := utf8.RuneCountInString(c.Check); n > maxWidth {
			maxWidth = n
		}
	}
	if maxWidth > 52 {
		maxWidth = 52
	}

	for _, c := range checks {
		pad := max(maxWidth-utf8.RuneCountInString(c.Check), 0)
		fmt.Fprintf(w, "  %s  %s%s  %s\n", reporter.StatusIcon(cs, c.Status), c.Check, strings.Repeat(" ", pad), c.Message)
		if c.Detail != "" {
			fmt.Fprintf(w, "        %s %s\n", cs.Muted("↳"), c.Detail)
		}
	}
}
