// Package aasa fetches, parses, and matches apple-app-site-association (AASA)
// files — the server-side half of iOS Universal Links.
//
// It replaces the ad-hoc map[string]interface{} walking that used to live in
// internal/validator with typed parsing (internal/models.AASAFile) plus a
// path/component matcher, so callers can answer "does this AASA file cover
// this exact URL" and not just "is this AASA file well-formed".
package aasa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/space-code/linkctl/internal/models"
)

// MaxSizeBytes is Apple's documented limit for the AASA file.
const MaxSizeBytes = 128 * 1024

// AppIDPattern matches the "<TeamID>.<BundleID>" shape of an appID.
var AppIDPattern = regexp.MustCompile(`^[A-Z0-9]{10}\.[a-zA-Z0-9_.-]+$`)

// Source identifies where an AASA file was retrieved from. iOS itself only
// ever reads from the Apple CDN in production (it fetches the well-known
// path once and Apple's infrastructure caches/serves it from there), so a
// difference between SourceWellKnown and SourceAppleCDN is a strong signal
// that changes haven't propagated yet or that Apple rejected the file.
type Source string

const (
	SourceWellKnown Source = "well-known" // https://<host>/.well-known/apple-app-site-association
	SourceRoot      Source = "root"       // https://<host>/apple-app-site-association (legacy fallback)
	SourceAppleCDN  Source = "apple-cdn"  // https://app-site-association.cdn-apple.com/a/v1/<host>
)

// Fetch is the raw outcome of retrieving one AASA file over HTTP.
type Fetch struct {
	Source      Source `json:"source"`
	URL         string `json:"url"`
	StatusCode  int    `json:"status_code,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	SizeBytes   int    `json:"size_bytes,omitempty"`
	Redirected  bool   `json:"redirected,omitempty"`
	Location    string `json:"location,omitempty"`
	TLSError    string `json:"tls_error,omitempty"`
	FetchError  string `json:"fetch_error,omitempty"`
	Body        []byte `json:"-"`
}

// wellKnownPath is the only path Apple's own client (and this tool's default
// behaviour) trusts; SourceRoot is kept only as a diagnostic aid because
// older integration guides sometimes described it.
const wellKnownPath = "/.well-known/apple-app-site-association"

func appleCDNURL(domain string) string {
	return "https://app-site-association.cdn-apple.com/a/v1/" + domain
}

// FetchAll retrieves the AASA file from every source worth checking:
// the well-known path, the legacy root path, and Apple's own CDN.
// Network/parse failures are recorded on the returned Fetch rather than
// returned as an error, so callers always get a full picture even when one
// source is unreachable.
func FetchAll(ctx context.Context, client *http.Client, domain string) []Fetch {
	domain = strings.TrimSuffix(domain, "/")

	sources := []struct {
		source Source
		url    string
	}{
		{SourceWellKnown, "https://" + domain + wellKnownPath},
		{SourceRoot, "https://" + domain + "/apple-app-site-association"},
		{SourceAppleCDN, appleCDNURL(domain)},
	}

	fetches := make([]Fetch, 0, len(sources))
	for _, s := range sources {
		fetches = append(fetches, fetchOne(ctx, client, s.source, s.url))
	}
	return fetches
}

// FetchOne retrieves a single named source. Exposed so `linkctl aasa
// --source well-known` can fetch only what it needs.
func FetchOne(ctx context.Context, client *http.Client, domain string, source Source) Fetch {
	var target string
	switch source {
	case SourceWellKnown:
		target = "https://" + strings.TrimSuffix(domain, "/") + wellKnownPath
	case SourceRoot:
		target = "https://" + strings.TrimSuffix(domain, "/") + "/apple-app-site-association"
	case SourceAppleCDN:
		target = appleCDNURL(strings.TrimSuffix(domain, "/"))
	default:
		return Fetch{Source: source, FetchError: fmt.Sprintf("unknown source %q", source)}
	}
	return fetchOne(ctx, client, source, target)
}

func fetchOne(ctx context.Context, client *http.Client, source Source, target string) Fetch {
	f := Fetch{Source: source, URL: target}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		f.FetchError = err.Error()
		return f
	}

	resp, err := client.Do(req)
	if err != nil {
		if isTLSError(err) {
			f.TLSError = err.Error()
		} else {
			f.FetchError = err.Error()
		}
		return f
	}
	defer resp.Body.Close()

	f.StatusCode = resp.StatusCode
	f.ContentType = resp.Header.Get("Content-Type")

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		f.Redirected = true
		f.Location = resp.Header.Get("Location")
		return f
	}

	if resp.StatusCode != http.StatusOK {
		return f
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxSizeBytes+1))
	if err != nil {
		f.FetchError = fmt.Sprintf("failed to read response body: %v", err)
		return f
	}

	f.SizeBytes = len(body)
	f.Body = body
	return f
}

func isTLSError(err error) bool {
	return strings.Contains(err.Error(), "x509") || strings.Contains(err.Error(), "tls:")
}

// Parse decodes an AASA JSON body into models.AASAFile.
//
// It accepts both shapes seen in the wild for applinks.details:
//   - the documented array form: "details": [ {"appID": "...", ...}, ... ]
//   - a legacy object form some older guides describe:
//     "details": { "<appID>": {"paths": [...]} , ... }
//
// In the legacy form the map key becomes the entry's AppID.
func Parse(body []byte) (*models.AASAFile, error) {
	var raw struct {
		AppLinks struct {
			Apps    []string        `json:"apps"`
			Details json.RawMessage `json:"details"`
		} `json:"applinks"`
		WebCredentials *struct {
			Apps []string `json:"apps"`
		} `json:"webcredentials,omitempty"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("AASA is not valid JSON: %w", err)
	}

	file := &models.AASAFile{}
	file.AppLinks.Apps = raw.AppLinks.Apps
	file.WebCredentials = raw.WebCredentials

	details, err := parseDetails(raw.AppLinks.Details)
	if err != nil {
		return nil, err
	}
	file.AppLinks.Details = details

	return file, nil
}

func parseDetails(rawDetails json.RawMessage) ([]models.AASADetail, error) {
	if len(rawDetails) == 0 {
		return nil, nil
	}

	// Try array form first (the documented, common shape).
	var asArray []models.AASADetail
	if err := json.Unmarshal(rawDetails, &asArray); err == nil {
		return asArray, nil
	}

	// Fall back to legacy object form: {"<appID>": {"paths": [...]}}.
	var asObject map[string]models.AASADetail
	if err := json.Unmarshal(rawDetails, &asObject); err != nil {
		return nil, fmt.Errorf("'details' field must be an array or object: %w", err)
	}
	details := make([]models.AASADetail, 0, len(asObject))
	for appID, d := range asObject {
		if d.AppID == "" {
			d.AppID = appID
		}
		details = append(details, d)
	}
	return details, nil
}

// MatchResult is the outcome of testing one URL against one AASA detail.
type MatchResult struct {
	Matched   bool
	Excluded  bool // true when a matching component/path explicitly excludes the URL
	Detail    *models.AASADetail
	Component *models.AASAComponent // set when the match/exclusion came from the modern "components" form
	Path      string                // set when the match/exclusion came from the legacy "paths" form
}

// MatchDetail tests u against a single AASADetail, preferring the modern
// "components" form when present (Apple ignores "paths" when "components"
// exists) and falling back to legacy "paths" glob matching otherwise.
// An entry with neither field present matches every path, per Apple's spec.
func MatchDetail(d models.AASADetail, u *url.URL) MatchResult {
	if len(d.Components) > 0 {
		return matchComponents(d, u)
	}
	if len(d.Paths) > 0 {
		return matchPaths(d, u)
	}
	// Neither present — matches all paths for this app.
	return MatchResult{Matched: true, Detail: &d}
}

// MatchAny tests u against every detail in file and returns the first
// (matched or excluded) result, mirroring how iOS evaluates entries in
// declaration order and stops at the first component/path that applies.
func MatchAny(file *models.AASAFile, u *url.URL) MatchResult {
	for i := range file.AppLinks.Details {
		d := file.AppLinks.Details[i]
		res := MatchDetail(d, u)
		if res.Matched || res.Excluded {
			return res
		}
	}
	return MatchResult{Matched: false}
}

func matchComponents(d models.AASADetail, u *url.URL) MatchResult {
	for i := range d.Components {
		c := d.Components[i]
		if componentMatches(c, u) {
			if c.Exclude {
				return MatchResult{Excluded: true, Detail: &d, Component: &c}
			}
			return MatchResult{Matched: true, Detail: &d, Component: &c}
		}
	}
	return MatchResult{}
}

func componentMatches(c models.AASAComponent, u *url.URL) bool {
	caseSensitive := true
	if c.CaseSensitive != nil {
		caseSensitive = *c.CaseSensitive
	}

	path := u.Path
	if c.PercentEncoded {
		path = u.EscapedPath()
	}

	if c.Path != "" && !globMatch(c.Path, path, caseSensitive) {
		return false
	}
	if c.Fragment != "" && !globMatch(c.Fragment, u.Fragment, caseSensitive) {
		return false
	}
	if c.Query != nil && !queryMatches(c.Query, u.Query(), caseSensitive) {
		return false
	}
	return true
}

// queryMatches supports both forms of the "?" key: a single glob pattern
// matched against the raw query string, or an object mapping individual
// parameter names to glob patterns (all must match; extra params on the URL
// are ignored, matching Apple's documented behaviour).
func queryMatches(pattern any, q url.Values, caseSensitive bool) bool {
	switch p := pattern.(type) {
	case string:
		return globMatch(p, q.Encode(), caseSensitive)
	case map[string]any:
		for key, v := range p {
			glob, ok := v.(string)
			if !ok {
				return false
			}
			if !globMatch(glob, q.Get(key), caseSensitive) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func matchPaths(d models.AASADetail, u *url.URL) MatchResult {
	for _, p := range d.Paths {
		exclude := strings.HasPrefix(p, "NOT ")
		pattern := strings.TrimPrefix(p, "NOT ")
		if globMatch(pattern, u.Path, true) {
			if exclude {
				return MatchResult{Excluded: true, Detail: &d, Path: p}
			}
			return MatchResult{Matched: true, Detail: &d, Path: p}
		}
	}
	return MatchResult{}
}

// globMatch implements Apple's AASA glob syntax: "*" matches any sequence of
// characters (including none, including "/"), "?" matches exactly one
// character. Everything else matches literally.
func globMatch(pattern, s string, caseSensitive bool) bool {
	if !caseSensitive {
		pattern = strings.ToLower(pattern)
		s = strings.ToLower(s)
	}
	return globMatchRunes([]rune(pattern), []rune(s))
}

func globMatchRunes(pattern, s []rune) bool {
	if len(pattern) == 0 {
		return len(s) == 0
	}

	switch pattern[0] {
	case '*':
		// Try matching the rest of the pattern at every possible split point.
		for i := 0; i <= len(s); i++ {
			if globMatchRunes(pattern[1:], s[i:]) {
				return true
			}
		}
		return false
	case '?':
		if len(s) == 0 {
			return false
		}
		return globMatchRunes(pattern[1:], s[1:])
	default:
		if len(s) == 0 || pattern[0] != s[0] {
			return false
		}
		return globMatchRunes(pattern[1:], s[1:])
	}
}
