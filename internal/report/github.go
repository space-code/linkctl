package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/space-code/linkctl/internal/models"
)

// WriteGitHub renders r as GitHub Actions workflow commands
// (https://docs.github.com/actions/using-workflows/workflow-commands-for-github-actions),
// so a FAIL/WARN check shows up as an inline annotation on the job summary
// and (for PR runs) in the "Files changed" / checks UI. PASS and INFO
// checks produce no annotation — CI logs stay readable when everything is
// fine, and only failures earn attention.
func WriteGitHub(w io.Writer, r *Report) {
	for _, section := range r.Sections {
		for _, c := range section.Checks {
			title := ghEscapeProperty(fmt.Sprintf("%s: %s", section.Name, c.Check))
			message := c.Message
			if c.Detail != "" {
				message += " — " + c.Detail
			}
			message = ghEscapeData(message)

			switch c.Status {
			case models.StatusFail:
				fmt.Fprintf(w, "::error title=%s::%s\n", title, message)
			case models.StatusWarning:
				fmt.Fprintf(w, "::warning title=%s::%s\n", title, message)
			}
		}
	}

	summary := r.Summary()
	fmt.Fprintf(
		w, "::notice title=%s::%d passed, %d failed, %d warnings\n",
		ghEscapeProperty(r.Command+" summary"), summary.Passed, summary.Failed, summary.Warnings,
	)
}

// ghEscapeData escapes a workflow-command value per GitHub's documented
// rules (applies to the message body after "::").
func ghEscapeData(s string) string {
	r := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	return r.Replace(s)
}

// ghEscapeProperty additionally escapes ':' and ',', which delimit
// workflow-command properties such as "title=...".
func ghEscapeProperty(s string) string {
	r := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
	return r.Replace(s)
}
