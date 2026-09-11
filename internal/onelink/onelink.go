// Package onelink analyzes AppsFlyer OneLink URLs: the template/shortlink
// structure, the deep-linking query parameters that determine what happens
// when the link is opened, whether the OneLink domain itself declares an
// AASA, and — by following the redirect chain — whether the domain the
// link actually lands on serves an AASA that covers it.
package onelink

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/resolve"
)

// Parameters AppsFlyer documents for OneLink deep linking behaviour.
// https://support.appsflyer.com/hc/en-us/articles/207447163
const (
	paramDeepLinkValue = "deep_link_value"
	paramAFDP          = "af_dp"     // custom-scheme deep link for the app
	paramAFWebDP       = "af_web_dp" // desktop/no-app fallback URL
	paramAFForceDeep   = "af_force_deeplink"
	paramMediaSource   = "pid"
	paramCampaign      = "c"
)

// Options configures Analyze.
type Options struct {
	// Want, when non-empty, is checked for presence in any AASA files found
	// along the way.
	Want aasa.AppIdentity

	// MaxHops caps the redirect trace (resolve.DefaultMaxHops when <= 0).
	MaxHops int

	// SkipResolve disables following the redirect chain, restricting
	// Analyze to static structure/parameter checks (faster, no dependence
	// on the link actually being live).
	SkipResolve bool
}

// Report is the full outcome of analyzing one OneLink URL.
type Report struct {
	Link        string                    `json:"link"`
	Domain      string                    `json:"domain"`
	Branded     bool                      `json:"branded"`
	TemplateID  string                    `json:"template_id,omitempty"`
	ShortlinkID string                    `json:"shortlink_id,omitempty"`
	Params      map[string]string         `json:"params,omitempty"`
	Trace       *resolve.Trace            `json:"trace,omitempty"`
	Checks      []models.ValidationResult `json:"checks"`
}

// Analyze parses and validates rawLink as an AppsFlyer OneLink URL.
func Analyze(ctx context.Context, client *http.Client, rawLink string, opts Options) (*Report, error) {
	u, err := url.Parse(rawLink)
	if err != nil {
		return nil, fmt.Errorf("invalid link: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("onelink requires an http(s) URL, got scheme %q", u.Scheme)
	}

	r := &Report{
		Link:    rawLink,
		Domain:  u.Host,
		Branded: !strings.HasSuffix(strings.ToLower(u.Host), ".onelink.me"),
		Params:  flattenQuery(u.Query()),
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) >= 1 && segments[0] != "" {
		r.TemplateID = segments[0]
	}
	if len(segments) >= 2 {
		r.ShortlinkID = segments[1]
	}

	r.Checks = append(r.Checks, checkStructure(r)...)
	r.Checks = append(r.Checks, checkParams(r.Params)...)
	r.Checks = append(r.Checks, checkDomainAASA(ctx, client, u, opts.Want)...)

	if !opts.SkipResolve {
		trace, err := resolve.Follow(ctx, client, rawLink, opts.MaxHops)
		if err != nil {
			return nil, fmt.Errorf("following redirect chain: %w", err)
		}
		r.Trace = trace
		r.Checks = append(r.Checks, prefixChecks("Redirect", resolve.Check(trace))...)
		r.Checks = append(r.Checks, checkFinalDestinationAASA(ctx, client, trace, opts.Want)...)
	}

	return r, nil
}

func checkStructure(r *Report) []models.ValidationResult {
	var results []models.ValidationResult

	if r.Branded {
		results = append(results, models.ValidationResult{
			Check:   "Domain",
			Status:  models.StatusInfo,
			Message: fmt.Sprintf("%q is a branded/custom OneLink domain", r.Domain),
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   "Domain",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("%q is a standard onelink.me subdomain", r.Domain),
		})
	}

	if r.TemplateID == "" {
		results = append(results, models.ValidationResult{
			Check:   "OneLink Template",
			Status:  models.StatusFail,
			Message: "no template ID found in the URL path",
			Detail:  "Expected https://<sub>.onelink.me/<templateID>/<shortlinkID>",
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   "OneLink Template",
			Status:  models.StatusPass,
			Message: r.TemplateID,
		})
	}

	return results
}

func checkParams(params map[string]string) []models.ValidationResult {
	var results []models.ValidationResult

	_, hasDeepLinkValue := params[paramDeepLinkValue]
	_, hasAFDP := params[paramAFDP]
	if !hasDeepLinkValue && !hasAFDP {
		results = append(results, models.ValidationResult{
			Check:   "Deep Link Target",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("neither %q nor %q is set", paramDeepLinkValue, paramAFDP),
			Detail:  "Without one of these the app opens to its home screen instead of the intended content",
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   "Deep Link Target",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("%s=%s", firstSetKey(params, paramDeepLinkValue, paramAFDP), firstSetValue(params, paramDeepLinkValue, paramAFDP)),
		})
	}

	if _, ok := params[paramAFWebDP]; !ok {
		results = append(results, models.ValidationResult{
			Check:   "Desktop Fallback",
			Status:  models.StatusWarning,
			Message: fmt.Sprintf("no %q param", paramAFWebDP),
			Detail:  "Users without the app installed, or on desktop, get AppsFlyer's default redirect instead of your own fallback page",
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   "Desktop Fallback",
			Status:  models.StatusPass,
			Message: params[paramAFWebDP],
		})
	}

	if _, ok := params[paramMediaSource]; !ok {
		results = append(results, models.ValidationResult{
			Check:   "Attribution",
			Status:  models.StatusWarning,
			Message: fmt.Sprintf("no %q (media source) param", paramMediaSource),
			Detail:  "Attribution reporting will be incomplete for this link",
		})
	}
	if _, ok := params[paramCampaign]; !ok {
		results = append(results, models.ValidationResult{
			Check:   "Campaign",
			Status:  models.StatusInfo,
			Message: fmt.Sprintf("no %q (campaign) param", paramCampaign),
		})
	}

	if v, ok := params[paramAFForceDeep]; ok && !strings.EqualFold(v, "true") {
		results = append(results, models.ValidationResult{
			Check:   "Force Deep Link",
			Status:  models.StatusWarning,
			Message: fmt.Sprintf("%s=%s (expected \"true\")", paramAFForceDeep, v),
		})
	}

	return results
}

// checkDomainAASA checks whether the OneLink domain itself serves an AASA.
// Most OneLink integrations use a JS smart-banner + redirect rather than a
// Universal Link on the onelink.me domain, so a missing AASA here is normal
// and reported as informational, not a failure — but if one is present it
// is validated fully, since a broken AASA that *was* set up deliberately is
// a real problem.
func checkDomainAASA(ctx context.Context, client *http.Client, u *url.URL, want aasa.AppIdentity) []models.ValidationResult {
	return checkAASALenient(ctx, client, u, want,
		"OneLink Domain AASA", "no AASA on the OneLink domain",
		"Expected for standard OneLink setups that redirect via JavaScript rather than serving a Universal Link directly")
}

// checkFinalDestinationAASA follows up on where the link actually lands:
// if the redirect chain ends on an http(s) page (not a custom-scheme deep
// link and not a dead end), that domain's AASA is checked for coverage of
// the final path — this is the check a plain `aasa`/`validate` run against
// the OneLink URL itself could never make, since the AASA lives elsewhere.
// A missing AASA here is informational too: the landing page may just be a
// plain marketing/fallback page with no Universal Link support at all.
func checkFinalDestinationAASA(ctx context.Context, client *http.Client, trace *resolve.Trace, want aasa.AppIdentity) []models.ValidationResult {
	if trace.Final == "" || len(trace.Hops) == 0 {
		return nil
	}
	if trace.Hops[len(trace.Hops)-1].Kind == resolve.HopKindCustomScheme {
		return nil // already a deep link — nothing further to check
	}
	if trace.FinalStatus >= 400 || trace.FinalStatus == 0 {
		return nil // dead end already reported by the redirect checks
	}

	finalURL, err := url.Parse(trace.Final)
	if err != nil || (finalURL.Scheme != "http" && finalURL.Scheme != "https") {
		return nil
	}

	return checkAASALenient(ctx, client, finalURL, want,
		"Final Domain AASA", "no AASA on the landing domain",
		"The link may simply land on a plain page with no Universal Link support")
}

// checkAASALenient fetches the well-known AASA at u.Host and validates it,
// treating a missing file (404/unreachable) as informational rather than a
// failure — appropriate for domains where an AASA is optional, unlike the
// primary domain a `validate`/`aasa` invocation targets directly.
func checkAASALenient(ctx context.Context, client *http.Client, u *url.URL, want aasa.AppIdentity, label, infoMessage, infoDetail string) []models.ValidationResult {
	fetch := aasa.FetchOne(ctx, client, u.Host, aasa.SourceWellKnown)

	if fetch.StatusCode == 0 || fetch.StatusCode == http.StatusNotFound {
		return []models.ValidationResult{{
			Check:   label,
			Status:  models.StatusInfo,
			Message: infoMessage,
			Detail:  infoDetail,
		}}
	}

	checks := aasa.Check([]aasa.Fetch{fetch}, aasa.CheckOptions{Want: want, TargetURL: u})
	return prefixChecks(label, checks)
}

func prefixChecks(prefix string, checks []models.ValidationResult) []models.ValidationResult {
	out := make([]models.ValidationResult, len(checks))
	for i, c := range checks {
		c.Check = prefix + ": " + c.Check
		out[i] = c
	}
	return out
}

func flattenQuery(q url.Values) map[string]string {
	if len(q) == 0 {
		return nil
	}
	out := make(map[string]string, len(q))
	for k, vs := range q {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out
}

func firstSetKey(params map[string]string, keys ...string) string {
	for _, k := range keys {
		if _, ok := params[k]; ok {
			return k
		}
	}
	return ""
}

func firstSetValue(params map[string]string, keys ...string) string {
	return params[firstSetKey(params, keys...)]
}
