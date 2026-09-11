// Package resolve traces the full redirect chain a link goes through
// before it reaches its real destination — the piece AppsFlyer OneLink
// links need that a plain AASA check can't provide, since the AASA file
// lives on the *final* domain while the link itself points at a template
// domain (app.onelink.me or a branded subdomain) several hops away.
//
// Unlike internal/aasa/internal/validator, which treat any redirect on the
// AASA endpoint itself as a failure, this package exists specifically to
// follow the chain: HTTP 3xx, HTML <meta http-equiv="refresh">, and simple
// JS location assignments, stopping when it reaches a custom-scheme URI
// (the deep link itself) or a page with no further redirect.
package resolve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/space-code/linkctl/internal/models"
)

// DefaultMaxHops caps how many redirects Follow will follow before giving up.
const DefaultMaxHops = 10

// maxBodyBytes limits how much of an HTML response Follow reads looking for
// a meta-refresh or JS redirect, so a huge page doesn't get read in full.
const maxBodyBytes = 256 * 1024

// HopKind classifies how a hop's destination was discovered.
type HopKind string

const (
	HopKindHTTP         HopKind = "http"           // 3xx status + Location header
	HopKindMetaRefresh  HopKind = "meta-refresh"   // <meta http-equiv="refresh">
	HopKindJS           HopKind = "js"             // window.location / location.replace(...)
	HopKindCustomScheme HopKind = "custom-scheme"  // terminal: a non-http(s) URI, the deep link itself
	HopKindFinal        HopKind = "final-response" // terminal: a 2xx/4xx/5xx with no further redirect found
)

// Hop is one step in the chain.
type Hop struct {
	URL        string        `json:"url"`
	Kind       HopKind       `json:"kind"`
	StatusCode int           `json:"status_code,omitempty"`
	Location   string        `json:"location,omitempty"`
	Duration   time.Duration `json:"duration_ms"`
	TLSError   string        `json:"tls_error,omitempty"`
	FetchError string        `json:"fetch_error,omitempty"`
}

// Trace is the full outcome of following a link's redirect chain.
type Trace struct {
	Start       string                    `json:"start"`
	Final       string                    `json:"final"`
	FinalStatus int                       `json:"final_status,omitempty"`
	Hops        []Hop                     `json:"hops"`
	Truncated   bool                      `json:"truncated,omitempty"`
	CycleAt     string                    `json:"cycle_at,omitempty"`
	Issues      []models.ValidationResult `json:"-"`
}

var (
	metaRefreshRe = regexp.MustCompile(`(?is)<meta[^>]+http-equiv\s*=\s*["']?refresh["']?[^>]*content\s*=\s*["']?\s*\d+\s*;\s*url\s*=\s*([^"'>\s]+)`)
	jsAssignRe    = regexp.MustCompile(`(?is)(?:window\.)?location(?:\.href)?\s*=\s*["']([^"']+)["']`)
	jsReplaceRe   = regexp.MustCompile(`(?is)location\.(?:replace|assign)\(\s*["']([^"']+)["']\s*\)`)
)

// Follow traces raw's redirect chain up to maxHops steps (DefaultMaxHops
// when <= 0). client should be built with httpx.Options{FollowRedirects:
// false} so each 3xx is observed individually rather than followed
// transparently.
func Follow(ctx context.Context, client *http.Client, raw string, maxHops int) (*Trace, error) {
	if maxHops <= 0 {
		maxHops = DefaultMaxHops
	}

	t := &Trace{Start: raw}
	visited := map[string]bool{}
	current := raw

	for step := 0; step < maxHops; step++ {
		if visited[current] {
			t.CycleAt = current
			t.Truncated = true
			t.Issues = append(t.Issues, models.ValidationResult{
				Check:   "Redirect Chain",
				Status:  models.StatusFail,
				Message: "redirect loop detected",
				Detail:  fmt.Sprintf("%q was visited twice", current),
			})
			t.Final = current
			return t, nil
		}
		visited[current] = true

		u, err := url.Parse(current)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q: %w", current, err)
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			t.Hops = append(t.Hops, Hop{URL: current, Kind: HopKindCustomScheme})
			t.Final = current
			return t, nil
		}

		next, hop := followOne(ctx, client, u)
		t.Hops = append(t.Hops, hop)

		if hop.FetchError != "" || hop.TLSError != "" {
			t.Final = current
			t.FinalStatus = hop.StatusCode
			t.Issues = append(t.Issues, models.ValidationResult{
				Check:   "Redirect Chain",
				Status:  models.StatusFail,
				Message: "failed to fetch a hop in the redirect chain",
				Detail:  hop.TLSError + hop.FetchError,
			})
			return t, nil
		}

		if next == "" {
			t.Final = current
			t.FinalStatus = hop.StatusCode
			return t, nil
		}
		current = next
	}

	t.Truncated = true
	t.Final = current
	t.Issues = append(t.Issues, models.ValidationResult{
		Check:   "Redirect Chain",
		Status:  models.StatusWarning,
		Message: fmt.Sprintf("stopped after %d hops without reaching a final destination", maxHops),
	})
	return t, nil
}

// followOne makes one request and returns the next URL to follow (resolved
// against u when relative), or "" when this hop is terminal.
func followOne(ctx context.Context, client *http.Client, u *url.URL) (next string, hop Hop) {
	hop = Hop{URL: u.String(), Kind: HopKindFinal}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		hop.FetchError = err.Error()
		return "", hop
	}

	start := time.Now()
	resp, err := client.Do(req)
	hop.Duration = time.Since(start)
	if err != nil {
		if isTLSError(err) {
			hop.TLSError = err.Error()
		} else {
			hop.FetchError = err.Error()
		}
		return "", hop
	}
	defer resp.Body.Close()

	hop.StatusCode = resp.StatusCode

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc == "" {
			return "", hop
		}
		hop.Kind = HopKindHTTP
		hop.Location = loc
		return resolveAgainst(u, loc), hop
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return "", hop
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", hop
	}

	if m := metaRefreshRe.FindSubmatch(body); m != nil {
		hop.Kind = HopKindMetaRefresh
		target := strings.Trim(string(m[1]), `"'`)
		hop.Location = target
		return resolveAgainst(u, target), hop
	}

	if m := jsReplaceRe.FindSubmatch(body); m != nil {
		hop.Kind = HopKindJS
		hop.Location = string(m[1])
		return resolveAgainst(u, string(m[1])), hop
	}
	if m := jsAssignRe.FindSubmatch(body); m != nil {
		hop.Kind = HopKindJS
		hop.Location = string(m[1])
		return resolveAgainst(u, string(m[1])), hop
	}

	return "", hop
}

// resolveAgainst resolves target relative to base, so a redirect chain can
// use relative Location headers or relative JS/meta-refresh targets.
func resolveAgainst(base *url.URL, target string) string {
	targetURL, err := url.Parse(target)
	if err != nil {
		return target
	}
	return base.ResolveReference(targetURL).String()
}

func isTLSError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "x509") || strings.Contains(msg, "tls:")
}
