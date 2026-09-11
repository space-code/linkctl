package aasa

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/space-code/linkctl/internal/models"
)

// AppIdentity is the app a caller expects to find registered in the AASA
// file — either half may be empty when the caller only wants structural
// checks (e.g. `linkctl aasa <domain>` with no --bundle-id/--team-id).
type AppIdentity struct {
	TeamID   string
	BundleID string
}

// AppID renders "<TeamID>.<BundleID>", or "" when either half is missing.
func (a AppIdentity) AppID() string {
	if a.TeamID == "" || a.BundleID == "" {
		return ""
	}
	return a.TeamID + "." + a.BundleID
}

// Empty reports whether no identity was supplied.
func (a AppIdentity) Empty() bool {
	return a.TeamID == "" && a.BundleID == ""
}

// CheckOptions configures Check.
type CheckOptions struct {
	// Want, when non-empty, is checked for presence among the file's app
	// identifiers.
	Want AppIdentity

	// TargetURL, when set, is matched against applinks.details so callers
	// learn whether this specific path is actually covered.
	TargetURL *url.URL
}

// Check runs the full AASA validation suite against fetches (as produced by
// FetchAll) and returns one models.ValidationResult per check, in the same
// PASS/FAIL/WARN/INFO vocabulary used across the rest of linkctl.
func Check(fetches []Fetch, opts CheckOptions) []models.ValidationResult {
	var results []models.ValidationResult

	wellKnown := findFetch(fetches, SourceWellKnown)
	if wellKnown == nil {
		return append(results, models.ValidationResult{
			Check:   "AASA Fetch",
			Status:  models.StatusFail,
			Message: "no request was made to the well-known AASA endpoint",
		})
	}

	results = append(results, checkFetchBasics(*wellKnown)...)
	if wellKnown.Body == nil {
		return results
	}

	file, err := Parse(wellKnown.Body)
	if err != nil {
		return append(results, models.ValidationResult{
			Check:   "AASA JSON",
			Status:  models.StatusFail,
			Message: err.Error(),
		})
	}
	results = append(results, models.ValidationResult{
		Check:   "AASA JSON",
		Status:  models.StatusPass,
		Message: "valid JSON",
	})

	results = append(results, checkAppsField(file)...)
	results = append(results, checkDetails(file, opts)...)

	if cdn := findFetch(fetches, SourceAppleCDN); cdn != nil {
		results = append(results, checkCDNConsistency(*wellKnown, *cdn)...)
	}

	return results
}

func findFetch(fetches []Fetch, source Source) *Fetch {
	for i := range fetches {
		if fetches[i].Source == source {
			return &fetches[i]
		}
	}
	return nil
}

func checkFetchBasics(f Fetch) []models.ValidationResult {
	var results []models.ValidationResult

	if f.TLSError != "" {
		return append(results, models.ValidationResult{
			Check:   "TLS Certificate",
			Status:  models.StatusFail,
			Message: "TLS certificate verification failed",
			Detail:  f.TLSError,
		})
	}
	results = append(results, models.ValidationResult{
		Check:   "TLS Certificate",
		Status:  models.StatusPass,
		Message: "valid certificate chain",
	})

	if f.FetchError != "" {
		return append(results, models.ValidationResult{
			Check:   "AASA Fetch",
			Status:  models.StatusFail,
			Message: "failed to fetch AASA file",
			Detail:  f.FetchError,
		})
	}

	if f.Redirected {
		return append(results, models.ValidationResult{
			Check:   "AASA Fetch",
			Status:  models.StatusFail,
			Message: "AASA endpoint returned a redirect",
			Detail:  fmt.Sprintf("Location: %s — Apple requires a direct 200 OK, no redirects", f.Location),
		})
	}

	if f.StatusCode != 200 {
		return append(results, models.ValidationResult{
			Check:   "AASA Fetch",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("unexpected HTTP status %d (expected 200)", f.StatusCode),
		})
	}
	results = append(results, models.ValidationResult{
		Check:   "AASA Fetch",
		Status:  models.StatusPass,
		Message: fmt.Sprintf("HTTP 200 OK (%s)", f.URL),
	})

	if !strings.Contains(f.ContentType, "application/json") {
		results = append(results, models.ValidationResult{
			Check:   "Content-Type",
			Status:  models.StatusWarning,
			Message: fmt.Sprintf("Content-Type is %q (expected application/json)", f.ContentType),
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   "Content-Type",
			Status:  models.StatusPass,
			Message: f.ContentType,
		})
	}

	if f.SizeBytes > MaxSizeBytes {
		results = append(results, models.ValidationResult{
			Check:   "File Size",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("%d bytes exceeds Apple's %d byte limit", f.SizeBytes, MaxSizeBytes),
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   "File Size",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("%d bytes", f.SizeBytes),
		})
	}

	return results
}

func checkAppsField(file *models.AASAFile) []models.ValidationResult {
	if len(file.AppLinks.Apps) > 0 {
		return []models.ValidationResult{{
			Check:   "applinks.apps",
			Status:  models.StatusFail,
			Message: "applinks.apps must be an empty array",
			Detail:  "A non-empty 'apps' array disables Universal Links entirely per Apple's spec",
		}}
	}
	return []models.ValidationResult{{
		Check:   "applinks.apps",
		Status:  models.StatusPass,
		Message: "empty, as required",
	}}
}

func checkDetails(file *models.AASAFile, opts CheckOptions) []models.ValidationResult {
	var results []models.ValidationResult

	if len(file.AppLinks.Details) == 0 {
		return append(results, models.ValidationResult{
			Check:   "applinks.details",
			Status:  models.StatusFail,
			Message: "no entries found",
		})
	}
	results = append(results, models.ValidationResult{
		Check:   "applinks.details",
		Status:  models.StatusPass,
		Message: fmt.Sprintf("%d entr%s found", len(file.AppLinks.Details), pluralY(len(file.AppLinks.Details))),
	})

	var allAppIDs []string
	for i, d := range file.AppLinks.Details {
		ids := d.AllAppIDs()
		if len(ids) == 0 {
			results = append(results, models.ValidationResult{
				Check:   fmt.Sprintf("Entry #%d appID", i),
				Status:  models.StatusFail,
				Message: "missing 'appID'/'appIDs'",
			})
			continue
		}
		for _, id := range ids {
			allAppIDs = append(allAppIDs, id)
			if !AppIDPattern.MatchString(id) {
				results = append(results, models.ValidationResult{
					Check:   fmt.Sprintf("Entry #%d appID", i),
					Status:  models.StatusWarning,
					Message: fmt.Sprintf("%q does not match <TeamID>.<BundleID>", id),
				})
			}
		}
		if len(d.Components) == 0 && len(d.Paths) == 0 {
			results = append(results, models.ValidationResult{
				Check:   fmt.Sprintf("Entry #%d Paths", i),
				Status:  models.StatusWarning,
				Message: "no 'components' or 'paths' — this entry matches every path",
			})
		}
	}

	if want := opts.Want.AppID(); want != "" {
		if containsFold(allAppIDs, want) {
			results = append(results, models.ValidationResult{
				Check:   "Expected App ID",
				Status:  models.StatusPass,
				Message: fmt.Sprintf("%q is registered", want),
			})
		} else {
			results = append(results, models.ValidationResult{
				Check:   "Expected App ID",
				Status:  models.StatusFail,
				Message: fmt.Sprintf("%q not found in AASA", want),
				Detail:  "Found: " + strings.Join(allAppIDs, ", "),
			})
		}
	}

	if opts.TargetURL != nil {
		results = append(results, checkTargetURL(file, opts.TargetURL)...)
	}

	return results
}

func checkTargetURL(file *models.AASAFile, target *url.URL) []models.ValidationResult {
	res := MatchAny(file, target)

	switch {
	case res.Excluded:
		detail := ""
		switch {
		case res.Component != nil:
			detail = fmt.Sprintf("excluded by component %q", res.Component.Path)
		case res.Path != "":
			detail = fmt.Sprintf("excluded by path pattern %q", res.Path)
		}
		return []models.ValidationResult{{
			Check:   "Path Match",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("%q is explicitly excluded from Universal Links", target.Path),
			Detail:  detail,
		}}
	case res.Matched:
		detail := ""
		switch {
		case res.Component != nil && res.Component.Path != "":
			detail = fmt.Sprintf("matched component %q", res.Component.Path)
		case res.Path != "":
			detail = fmt.Sprintf("matched path pattern %q", res.Path)
		default:
			detail = "matched (entry has no path restriction)"
		}
		return []models.ValidationResult{{
			Check:   "Path Match",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("%q is covered by applinks.details", target.Path),
			Detail:  detail,
		}}
	default:
		return []models.ValidationResult{{
			Check:   "Path Match",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("%q is not covered by any applinks.details entry", target.Path),
			Detail:  "This link will open in the browser instead of the app",
		}}
	}
}

func checkCDNConsistency(wellKnown, cdn Fetch) []models.ValidationResult {
	if cdn.TLSError != "" || cdn.FetchError != "" {
		return []models.ValidationResult{{
			Check:   "Apple CDN",
			Status:  models.StatusWarning,
			Message: "could not reach Apple's AASA CDN to cross-check",
			Detail:  cdn.TLSError + cdn.FetchError,
		}}
	}
	if cdn.StatusCode != 200 || cdn.Body == nil {
		return []models.ValidationResult{{
			Check:   "Apple CDN",
			Status:  models.StatusWarning,
			Message: fmt.Sprintf("Apple's CDN returned HTTP %d — the file may not be cached there yet", cdn.StatusCode),
			Detail:  "This is what iOS devices actually fetch; propagation can take up to 24 hours after a DNS/hosting change",
		}}
	}

	cdnFile, err := Parse(cdn.Body)
	if err != nil {
		return []models.ValidationResult{{
			Check:   "Apple CDN",
			Status:  models.StatusWarning,
			Message: "Apple's CDN response is not valid AASA JSON",
			Detail:  err.Error(),
		}}
	}
	wkFile, err := Parse(wellKnown.Body)
	if err != nil {
		return nil // already reported by checkDetails via the well-known parse
	}

	if reflect.DeepEqual(cdnFile, wkFile) {
		return []models.ValidationResult{{
			Check:   "Apple CDN",
			Status:  models.StatusPass,
			Message: "matches your well-known endpoint",
		}}
	}
	return []models.ValidationResult{{
		Check:   "Apple CDN",
		Status:  models.StatusWarning,
		Message: "Apple's CDN is serving a different AASA than your well-known endpoint",
		Detail:  "Changes can take up to 24 hours to propagate; devices use the CDN copy, not your live server",
	}}
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func containsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}
