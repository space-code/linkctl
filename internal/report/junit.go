package report

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/space-code/linkctl/internal/models"
)

// One <testsuite> per Section — for a single-link command (aasa, resolve,
// onelink) that's one suite per concern; for `ci`, one suite per configured
// link, which is exactly the granularity GitLab/Jenkins test reports expect.
//
// FAIL checks become <failure>, WARN checks are recorded as passing but with
// a <system-out> note (a warning shouldn't fail the CI job on its own —
// that mirors this tool's own PASS/FAIL/WARN semantics, where only FAIL
// affects the exit code), and everything else is a plain passing <testcase>.
type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Failure   *junitMessage `xml:"failure,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitMessage struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// WriteJUnit renders r as JUnit XML.
func WriteJUnit(w io.Writer, r *Report) error {
	suites := junitSuites{Name: "linkctl " + r.Command}

	for _, section := range r.Sections {
		suite := junitSuite{Name: section.Name}
		for _, c := range section.Checks {
			suite.Tests++
			suites.Tests++

			tc := junitCase{Name: c.Check, ClassName: section.Name}
			switch c.Status {
			case models.StatusFail:
				suite.Failures++
				suites.Failures++
				tc.Failure = &junitMessage{Message: c.Message, Text: c.Detail}
			case models.StatusWarning:
				tc.SystemOut = fmt.Sprintf("WARN: %s", joinMessage(c.Message, c.Detail))
			case models.StatusInfo, models.StatusSkip:
				tc.SystemOut = joinMessage(c.Message, c.Detail)
			}
			suite.Cases = append(suite.Cases, tc)
		}
		suites.Suites = append(suites.Suites, suite)
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(suites); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func joinMessage(message, detail string) string {
	if detail == "" {
		return message
	}
	return message + " — " + detail
}
