// Package validator implements the `validate` command's checks.
//
// It is a thin adapter over internal/aasa: it fetches only the well-known
// AASA endpoint (unlike `aasa`, which also cross-checks the legacy root
// path and Apple's CDN) and flattens the richer PASS/FAIL/WARN/INFO check
// list into the simpler error/warning/info Issue vocabulary this command
// has always exposed, so existing --json consumers keep working.
package validator

import (
	"context"
	"fmt"
	"net/url"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/httpx"
	"github.com/space-code/linkctl/internal/models"
)

// Issue is one problem found during validation.
type Issue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Result is the outcome of validating one deep link.
type Result struct {
	URL        string  `json:"url"`
	Domain     string  `json:"domain"`
	StatusCode int     `json:"status_code,omitempty"`
	Issues     []Issue `json:"issues"`
	Valid      bool    `json:"valid"`
}

// HasErrors reports whether any issue is at error level.
func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Level == "error" {
			return true
		}
	}
	return false
}

// Options configures ValidateDeepLink.
type Options struct {
	// Insecure disables TLS certificate verification. Off by default —
	// Universal Links require a valid certificate.
	Insecure bool
}

// ValidateDeepLink validates rawURL's server-side deep link configuration.
//
// A non-http(s) scheme (custom URI schemes like "myapp://") short-circuits
// to a single informational issue, since there is nothing server-side to
// check. Otherwise it fetches apple-app-site-association over HTTPS,
// verifies the certificate, the HTTP response, the JSON structure, and —
// when rawURL has a path/query/fragment beyond the bare domain — whether
// that exact link is covered by applinks.details.
func ValidateDeepLink(rawURL string, opts Options) (*Result, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL: %s", rawURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &Result{
			URL:   rawURL,
			Valid: true,
			Issues: []Issue{{
				Level:   "info",
				Message: "Custom scheme detected, no server-side validation needed",
			}},
		}, nil
	}

	result := &Result{URL: rawURL, Domain: parsedURL.Hostname(), Valid: true}

	client := httpx.New(httpx.Options{
		FollowRedirects: false,
		Insecure:        opts.Insecure,
		UserAgent:       httpx.UserAgentIOS,
	})

	fetch := aasa.FetchOne(context.Background(), client, parsedURL.Host, aasa.SourceWellKnown)
	result.StatusCode = fetch.StatusCode

	var targetURL *url.URL
	if hasPathToMatch(parsedURL) {
		targetURL = parsedURL
	}

	checks := aasa.Check([]aasa.Fetch{fetch}, aasa.CheckOptions{TargetURL: targetURL})
	result.Issues = issuesFromChecks(checks)
	result.Valid = !result.HasErrors()

	return result, nil
}

// hasPathToMatch reports whether u carries more than a bare domain.
func hasPathToMatch(u *url.URL) bool {
	return (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != ""
}

// issuesFromChecks flattens aasa's PASS/FAIL/WARN/INFO checks into the
// error/warning/info Issue vocabulary; PASS/SKIP checks produce no issue,
// matching the original behaviour where only problems were reported.
func issuesFromChecks(checks []models.ValidationResult) []Issue {
	var issues []Issue
	for _, c := range checks {
		var level string
		switch c.Status {
		case models.StatusFail:
			level = "error"
		case models.StatusWarning:
			level = "warning"
		case models.StatusInfo:
			level = "info"
		default:
			continue
		}
		message := c.Message
		if c.Detail != "" {
			message = fmt.Sprintf("%s (%s)", message, c.Detail)
		}
		issues = append(issues, Issue{Level: level, Message: message})
	}
	return issues
}
