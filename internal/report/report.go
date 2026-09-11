// Package report is the shared output model for the network/CI-facing
// commands (aasa, resolve, onelink, ci): one Report holds one or more named
// Sections of checks, and can render itself as human-readable text, JSON,
// GitHub Actions annotations, or JUnit XML — the four formats a CI pipeline
// is likely to want (terminal for local runs, JSON for scripting, GitHub
// annotations for PR inline errors, JUnit for GitLab/Jenkins test reports).
package report

import (
	"slices"
	"time"

	"github.com/space-code/linkctl/internal/models"
)

// Format names accepted by the --format flag across commands that use this
// package.
const (
	FormatText   = "text"
	FormatJSON   = "json"
	FormatGitHub = "github"
	FormatJUnit  = "junit"
)

// ValidFormats lists every accepted --format value, for flag help text and
// validation.
var ValidFormats = []string{FormatText, FormatJSON, FormatGitHub, FormatJUnit}

// IsValidFormat reports whether name is one of ValidFormats.
func IsValidFormat(name string) bool {
	return slices.Contains(ValidFormats, name)
}

// Report is one command's full output: a target (a link, a domain, "ci run"),
// grouped into named sections, each carrying informational key/value rows
// and/or a list of pass/fail checks.
type Report struct {
	Command     string    `json:"command"`
	Target      string    `json:"target,omitempty"`
	Sections    []Section `json:"sections"`
	GeneratedAt time.Time `json:"generated_at"`
}

// Section is one logical group within a Report. For a single-link command
// (aasa, resolve, onelink) a Report typically has one Section per concern
// (e.g. "AASA", "Redirect Trace"). For a multi-link command (ci) a Report
// has one Section per configured link, named after that link.
type Section struct {
	Name   string                    `json:"name"`
	KV     [][2]string               `json:"info,omitempty"`
	Checks []models.ValidationResult `json:"checks,omitempty"`
}

// New creates an empty Report stamped with the current time.
func New(command, target string) *Report {
	return &Report{
		Command:     command,
		Target:      target,
		GeneratedAt: time.Now().UTC(),
	}
}

// AddSection appends a new Section and returns it by value; use AddChecks/
// AddKV via the returned index, or build the Section separately and pass it
// to Append. Kept simple (index-returning) so callers can keep mutating a
// section while building up a report in a loop.
func (r *Report) AddSection(name string) int {
	r.Sections = append(r.Sections, Section{Name: name})
	return len(r.Sections) - 1
}

// AddChecks appends checks to the section at index i.
func (r *Report) AddChecks(i int, checks ...models.ValidationResult) {
	r.Sections[i].Checks = append(r.Sections[i].Checks, checks...)
}

// AddKV appends a key/value info row to the section at index i.
func (r *Report) AddKV(i int, key, value string) {
	r.Sections[i].KV = append(r.Sections[i].KV, [2]string{key, value})
}

// AllChecks flattens every check across every section, in order.
func (r *Report) AllChecks() []models.ValidationResult {
	var all []models.ValidationResult
	for _, s := range r.Sections {
		all = append(all, s.Checks...)
	}
	return all
}

// Summary tallies AllChecks into pass/fail/warning counts.
func (r *Report) Summary() models.JSONSummary {
	var s models.JSONSummary
	for _, c := range r.AllChecks() {
		s.Total++
		switch c.Status {
		case models.StatusPass:
			s.Passed++
		case models.StatusFail:
			s.Failed++
		case models.StatusWarning:
			s.Warnings++
		}
	}
	s.OK = s.Failed == 0
	return s
}

// OK reports whether every check passed (no FAIL-status checks). A Report
// with zero checks is considered OK — an empty aggregate (e.g. `ci` with no
// configured links) is a configuration question, not a check failure.
func (r *Report) OK() bool {
	return r.Summary().Failed == 0
}
