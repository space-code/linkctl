package aasa_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/models"
)

func findResult(results []models.ValidationResult, check string) *models.ValidationResult {
	for i := range results {
		if results[i].Check == check {
			return &results[i]
		}
	}
	return nil
}

func hasStatus(results []models.ValidationResult, check string, status models.Status) bool {
	r := findResult(results, check)
	return r != nil && r.Status == status
}

func wellKnownFetch(status int, body string) aasa.Fetch {
	return aasa.Fetch{
		Source:      aasa.SourceWellKnown,
		URL:         "https://example.com/.well-known/apple-app-site-association",
		StatusCode:  status,
		ContentType: "application/json",
		SizeBytes:   len(body),
		Body:        []byte(body),
	}
}

func TestCheck_NoWellKnownFetch(t *testing.T) {
	results := aasa.Check(nil, aasa.CheckOptions{})
	if !hasStatus(results, "AASA Fetch", models.StatusFail) {
		t.Errorf("expected a FAIL 'AASA Fetch' result, got %+v", results)
	}
}

// Regression test: `linkctl aasa --source apple-cdn` (or --source root)
// fetches only that one source — Check must validate it directly instead
// of assuming a well-known fetch is always present.
func TestCheck_AppleCDNOnly_NoWellKnownFetch(t *testing.T) {
	body := `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`
	fetches := []aasa.Fetch{{
		Source:      aasa.SourceAppleCDN,
		StatusCode:  200,
		ContentType: "application/json",
		SizeBytes:   len(body),
		Body:        []byte(body),
	}}

	results := aasa.Check(fetches, aasa.CheckOptions{})

	if hasStatus(results, "AASA Fetch", models.StatusFail) {
		t.Errorf("did not expect a FAIL AASA Fetch result when apple-cdn was the only requested source, got %+v", results)
	}
	if !hasStatus(results, "AASA Fetch", models.StatusPass) {
		t.Errorf("expected a PASS AASA Fetch result derived from the apple-cdn fetch, got %+v", results)
	}
	if !hasStatus(results, "applinks.details", models.StatusPass) {
		t.Errorf("expected details to be validated from the apple-cdn fetch, got %+v", results)
	}
}

func TestCheck_RootOnly_NoWellKnownFetch(t *testing.T) {
	body := `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`
	fetches := []aasa.Fetch{{
		Source:      aasa.SourceRoot,
		StatusCode:  200,
		ContentType: "application/json",
		SizeBytes:   len(body),
		Body:        []byte(body),
	}}

	results := aasa.Check(fetches, aasa.CheckOptions{})

	if !hasStatus(results, "AASA Fetch", models.StatusPass) {
		t.Errorf("expected a PASS AASA Fetch result derived from the root fetch, got %+v", results)
	}
}

func TestCheck_HappyPath(t *testing.T) {
	body := `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["/profile/*"]}]}}`
	fetches := []aasa.Fetch{wellKnownFetch(200, body)}

	results := aasa.Check(fetches, aasa.CheckOptions{})

	for _, check := range []string{"TLS Certificate", "AASA Fetch", "AASA JSON", "applinks.apps", "applinks.details"} {
		if !hasStatus(results, check, models.StatusPass) {
			t.Errorf("expected PASS for %q, got %+v", check, findResult(results, check))
		}
	}
}

func TestCheck_TLSError(t *testing.T) {
	fetches := []aasa.Fetch{{Source: aasa.SourceWellKnown, TLSError: "x509: certificate signed by unknown authority"}}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "TLS Certificate", models.StatusFail) {
		t.Errorf("expected FAIL for TLS Certificate, got %+v", results)
	}
}

func TestCheck_Redirect(t *testing.T) {
	fetches := []aasa.Fetch{{Source: aasa.SourceWellKnown, Redirected: true, Location: "/other"}}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "AASA Fetch", models.StatusFail) {
		t.Errorf("expected FAIL for a redirected AASA fetch, got %+v", results)
	}
}

func TestCheck_NotFound(t *testing.T) {
	fetches := []aasa.Fetch{{Source: aasa.SourceWellKnown, StatusCode: 404}}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "AASA Fetch", models.StatusFail) {
		t.Errorf("expected FAIL for 404, got %+v", results)
	}
}

func TestCheck_WrongContentType(t *testing.T) {
	f := wellKnownFetch(200, `{"applinks":{"apps":[],"details":[]}}`)
	f.ContentType = "text/plain"
	results := aasa.Check([]aasa.Fetch{f}, aasa.CheckOptions{})
	if !hasStatus(results, "Content-Type", models.StatusWarning) {
		t.Errorf("expected WARN for wrong content type, got %+v", results)
	}
}

func TestCheck_OversizedFile(t *testing.T) {
	f := wellKnownFetch(200, `{"applinks":{"apps":[],"details":[]}}`)
	f.SizeBytes = aasa.MaxSizeBytes + 1
	results := aasa.Check([]aasa.Fetch{f}, aasa.CheckOptions{})
	if !hasStatus(results, "File Size", models.StatusFail) {
		t.Errorf("expected FAIL for oversized file, got %+v", results)
	}
}

func TestCheck_MalformedJSON(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{not json`)}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "AASA JSON", models.StatusFail) {
		t.Errorf("expected FAIL for malformed JSON, got %+v", results)
	}
}

func TestCheck_NonEmptyAppsBreaksUniversalLinks(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"apps":["some-app-id"],"details":[]}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "applinks.apps", models.StatusFail) {
		t.Errorf("expected FAIL for non-empty applinks.apps, got %+v", results)
	}
}

func TestCheck_LegacyObjectDetails(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":{"ABCDE12345.com.example.app":{"paths":["*"]}}}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "applinks.details", models.StatusPass) {
		t.Errorf("expected PASS for legacy object-form details, got %+v", results)
	}
}

func TestCheck_EmptyDetails(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"apps":[],"details":[]}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "applinks.details", models.StatusFail) {
		t.Errorf("expected FAIL for empty details, got %+v", results)
	}
}

func TestCheck_MalformedAppID(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"appID":"not-a-valid-appid","paths":["*"]}]}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "Entry #0 appID", models.StatusWarning) {
		t.Errorf("expected WARN for malformed appID, got %+v", results)
	}
}

func TestCheck_MissingAppID(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"paths":["*"]}]}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "Entry #0 appID", models.StatusFail) {
		t.Errorf("expected FAIL for missing appID, got %+v", results)
	}
}

func TestCheck_NoPathsOrComponentsWarns(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app"}]}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "Entry #0 Paths", models.StatusWarning) {
		t.Errorf("expected WARN for an entry with no path restriction, got %+v", results)
	}
}

func TestCheck_ExpectedAppIDFound(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{Want: aasa.AppIdentity{TeamID: "ABCDE12345", BundleID: "com.example.app"}})
	if !hasStatus(results, "Expected App ID", models.StatusPass) {
		t.Errorf("expected PASS for a found app ID, got %+v", results)
	}
}

func TestCheck_ExpectedAppIDMissing(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)}
	results := aasa.Check(fetches, aasa.CheckOptions{Want: aasa.AppIdentity{TeamID: "ZZZZZ99999", BundleID: "com.other.app"}})
	if !hasStatus(results, "Expected App ID", models.StatusFail) {
		t.Errorf("expected FAIL for a missing app ID, got %+v", results)
	}
}

func TestCheck_TargetURLMatched(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["/profile/*"]}]}}`)}
	target, _ := url.Parse("https://example.com/profile/42")
	results := aasa.Check(fetches, aasa.CheckOptions{TargetURL: target})
	if !hasStatus(results, "Path Match", models.StatusPass) {
		t.Errorf("expected PASS for a covered path, got %+v", results)
	}
}

func TestCheck_TargetURLNotCovered(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["/profile/*"]}]}}`)}
	target, _ := url.Parse("https://example.com/settings")
	results := aasa.Check(fetches, aasa.CheckOptions{TargetURL: target})
	if !hasStatus(results, "Path Match", models.StatusFail) {
		t.Errorf("expected FAIL for an uncovered path, got %+v", results)
	}
}

func TestCheck_TargetURLExcluded(t *testing.T) {
	fetches := []aasa.Fetch{wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["NOT /admin/*","/*"]}]}}`)}
	target, _ := url.Parse("https://example.com/admin/secret")
	results := aasa.Check(fetches, aasa.CheckOptions{TargetURL: target})
	if !hasStatus(results, "Path Match", models.StatusFail) {
		t.Errorf("expected FAIL for an excluded path, got %+v", results)
	}
}

func TestCheck_AppleCDNMatches(t *testing.T) {
	body := `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`
	fetches := []aasa.Fetch{
		wellKnownFetch(200, body),
		{Source: aasa.SourceAppleCDN, StatusCode: 200, Body: []byte(body)},
	}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "Apple CDN", models.StatusPass) {
		t.Errorf("expected PASS when CDN matches well-known, got %+v", results)
	}
}

func TestCheck_AppleCDNDiffers(t *testing.T) {
	fetches := []aasa.Fetch{
		wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["/new/*"]}]}}`),
		{Source: aasa.SourceAppleCDN, StatusCode: 200, Body: []byte(`{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["/old/*"]}]}}`)},
	}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "Apple CDN", models.StatusWarning) {
		t.Errorf("expected WARN when CDN differs from well-known, got %+v", results)
	}
}

func TestCheck_AppleCDNUnreachable(t *testing.T) {
	fetches := []aasa.Fetch{
		wellKnownFetch(200, `{"applinks":{"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`),
		{Source: aasa.SourceAppleCDN, FetchError: "connection refused"},
	}
	results := aasa.Check(fetches, aasa.CheckOptions{})
	if !hasStatus(results, "Apple CDN", models.StatusWarning) {
		t.Errorf("expected WARN when CDN is unreachable, got %+v", results)
	}
}

// End-to-end sanity check driving FetchAll + Check together against a real
// httptest server, to make sure the pieces compose correctly.
func TestFetchAllAndCheck_EndToEnd(t *testing.T) {
	body := `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["/profile/*"]}]}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer ts.Close()

	fetches := aasa.FetchAll(context.Background(), clientForServer(ts), "example.com")
	results := aasa.Check(fetches, aasa.CheckOptions{
		Want: aasa.AppIdentity{TeamID: "ABCDE12345", BundleID: "com.example.app"},
	})

	if !hasStatus(results, "AASA Fetch", models.StatusPass) {
		t.Errorf("expected AASA Fetch to pass, got %+v", results)
	}
	if !hasStatus(results, "Expected App ID", models.StatusPass) {
		t.Errorf("expected Expected App ID to pass, got %+v", results)
	}
}
