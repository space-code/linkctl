package report_test

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/report"
	"github.com/space-code/linkctl/pkg/iostreams"
)

func sampleReport() *report.Report {
	r := report.New("aasa", "https://example.com/profile/42")

	i := r.AddSection("AASA")
	r.AddKV(i, "Domain", "example.com")
	r.AddChecks(
		i,
		models.ValidationResult{Check: "AASA Fetch", Status: models.StatusPass, Message: "HTTP 200 OK"},
		models.ValidationResult{Check: "Path Match", Status: models.StatusFail, Message: "not covered", Detail: "add a components entry"},
		models.ValidationResult{Check: "Content-Type", Status: models.StatusWarning, Message: "unexpected content type"},
	)

	return r
}

func TestReport_Summary(t *testing.T) {
	r := sampleReport()
	s := r.Summary()

	if s.Total != 3 || s.Passed != 1 || s.Failed != 1 || s.Warnings != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if s.OK {
		t.Error("expected OK=false when a check failed")
	}
}

func TestReport_OK_NoChecks(t *testing.T) {
	r := report.New("aasa", "https://example.com")
	if !r.OK() {
		t.Error("expected an empty report to be OK")
	}
}

func TestReport_OK_AllPassed(t *testing.T) {
	r := report.New("aasa", "https://example.com")
	i := r.AddSection("AASA")
	r.AddChecks(i, models.ValidationResult{Check: "x", Status: models.StatusPass})
	if !r.OK() {
		t.Error("expected OK=true when nothing failed")
	}
}

func TestIsValidFormat(t *testing.T) {
	for _, f := range []string{"text", "json", "github", "junit"} {
		if !report.IsValidFormat(f) {
			t.Errorf("expected %q to be valid", f)
		}
	}
	if report.IsValidFormat("yaml") {
		t.Error("expected an unknown format to be invalid")
	}
}

func TestWriteText_ContainsExpectedContent(t *testing.T) {
	var buf bytes.Buffer
	ios, _, _, _ := iostreams.Test()
	report.WriteText(&buf, ios.ColorScheme(), sampleReport())

	out := buf.String()
	for _, want := range []string{"https://example.com/profile/42", "AASA", "AASA Fetch", "Path Match", "not covered", "add a components entry", "1 passed", "1 failed", "1 warnings"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected text output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestWriteJSON_RoundTrips(t *testing.T) {
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf, sampleReport()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded report.Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, buf.String())
	}

	if decoded.Command != "aasa" || decoded.Target != "https://example.com/profile/42" {
		t.Errorf("unexpected decoded report: %+v", decoded)
	}
	if len(decoded.Sections) != 1 || len(decoded.Sections[0].Checks) != 3 {
		t.Errorf("expected 1 section with 3 checks, got %+v", decoded.Sections)
	}
}

func TestWriteGitHub_AnnotatesFailuresAndWarnings(t *testing.T) {
	var buf bytes.Buffer
	report.WriteGitHub(&buf, sampleReport())

	out := buf.String()
	if !strings.Contains(out, "::error title=AASA%3A Path Match::not covered") {
		t.Errorf("expected an ::error:: annotation for the FAIL check, got:\n%s", out)
	}
	if !strings.Contains(out, "::warning title=AASA%3A Content-Type::unexpected content type") {
		t.Errorf("expected a ::warning:: annotation for the WARN check, got:\n%s", out)
	}
	if strings.Contains(out, "AASA Fetch") {
		t.Errorf("did not expect an annotation for a PASS check, got:\n%s", out)
	}
	if !strings.Contains(out, "::notice title=aasa summary::1 passed, 1 failed, 1 warnings") {
		t.Errorf("expected a summary notice, got:\n%s", out)
	}
}

func TestWriteGitHub_EscapesNewlinesInMessage(t *testing.T) {
	r := report.New("aasa", "https://example.com")
	i := r.AddSection("AASA")
	r.AddChecks(i, models.ValidationResult{Check: "x", Status: models.StatusFail, Message: "line one\nline two"})

	var buf bytes.Buffer
	report.WriteGitHub(&buf, r)

	if strings.Contains(buf.String(), "line one\nline two") {
		t.Error("expected the literal newline to be escaped as %0A")
	}
	if !strings.Contains(buf.String(), "line one%0Aline two") {
		t.Errorf("expected escaped newline in output, got:\n%s", buf.String())
	}
}

func TestWriteJUnit_ValidXMLWithFailureAndWarning(t *testing.T) {
	var buf bytes.Buffer
	if err := report.WriteJUnit(&buf, sampleReport()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded struct {
		XMLName  xml.Name `xml:"testsuites"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Suites   []struct {
			Name  string `xml:"name,attr"`
			Cases []struct {
				Name    string `xml:"name,attr"`
				Failure *struct {
					Message string `xml:"message,attr"`
				} `xml:"failure"`
				SystemOut string `xml:"system-out"`
			} `xml:"testcase"`
		} `xml:"testsuite"`
	}

	if err := xml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid XML: %v\ngot: %s", err, buf.String())
	}

	if decoded.Tests != 3 || decoded.Failures != 1 {
		t.Errorf("expected tests=3 failures=1, got tests=%d failures=%d", decoded.Tests, decoded.Failures)
	}
	if len(decoded.Suites) != 1 || len(decoded.Suites[0].Cases) != 3 {
		t.Fatalf("expected 1 suite with 3 cases, got %+v", decoded.Suites)
	}

	var sawFailure, sawWarningNote bool
	for _, c := range decoded.Suites[0].Cases {
		if c.Name == "Path Match" && c.Failure != nil {
			sawFailure = true
		}
		if c.Name == "Content-Type" && strings.Contains(c.SystemOut, "WARN") {
			sawWarningNote = true
		}
	}
	if !sawFailure {
		t.Error("expected the FAIL check to render as <failure>")
	}
	if !sawWarningNote {
		t.Error("expected the WARN check to render as a <system-out> note, not a failure")
	}
}

func TestWriteJUnit_MultipleSuitesForMultipleSections(t *testing.T) {
	r := report.New("ci", "")
	i1 := r.AddSection("https://a.example.com")
	r.AddChecks(i1, models.ValidationResult{Check: "x", Status: models.StatusPass})
	i2 := r.AddSection("https://b.example.com")
	r.AddChecks(i2, models.ValidationResult{Check: "y", Status: models.StatusFail, Message: "boom"})

	var buf bytes.Buffer
	if err := report.WriteJUnit(&buf, r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded struct {
		Suites []struct {
			Name string `xml:"name,attr"`
		} `xml:"testsuite"`
	}
	if err := xml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}
	if len(decoded.Suites) != 2 {
		t.Fatalf("expected 2 testsuites (one per link), got %d", len(decoded.Suites))
	}
}
