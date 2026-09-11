package onelink_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/httpx"
	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/onelink"
)

func newClient() *http.Client {
	return httpx.New(httpx.Options{FollowRedirects: false, UserAgent: httpx.UserAgentIOS})
}

// newInsecureClient is used against httptest.NewTLSServer (self-signed
// certs), since AASA is always fetched over https.
func newInsecureClient() *http.Client {
	return httpx.New(httpx.Options{FollowRedirects: false, UserAgent: httpx.UserAgentIOS, Insecure: true})
}

func findCheck(results []models.ValidationResult, check string) *models.ValidationResult {
	for i := range results {
		if results[i].Check == check {
			return &results[i]
		}
	}
	return nil
}

func TestAnalyze_ParsesTemplateAndShortlink(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // no AASA on this domain, no redirect target
	}))
	defer ts.Close()

	link := ts.URL + "/abc1/xyz789?deep_link_value=profile42&af_web_dp=https://example.com&pid=google&c=summer"
	r, err := onelink.Analyze(context.Background(), newClient(), link, onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.TemplateID != "abc1" || r.ShortlinkID != "xyz789" {
		t.Errorf("expected template=abc1 shortlink=xyz789, got template=%q shortlink=%q", r.TemplateID, r.ShortlinkID)
	}
}

func TestAnalyze_BrandedDomainDetection(t *testing.T) {
	client := httpx.New(httpx.Options{Timeout: 2 * time.Second})
	r, err := onelink.Analyze(context.Background(), client, "https://links.mybrand.invalid/abc/xyz", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Branded {
		t.Error("expected a non-onelink.me domain to be detected as branded")
	}
}

func TestAnalyze_StandardOnelinkDomainNotBranded(t *testing.T) {
	r, err := onelink.Analyze(context.Background(), newClient(), "https://example.onelink.me/abc/xyz", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Branded {
		t.Error("expected a *.onelink.me domain to not be branded")
	}
}

func TestAnalyze_MissingTemplateID(t *testing.T) {
	r, err := onelink.Analyze(context.Background(), newClient(), "https://example.onelink.me/", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "OneLink Template")
	if c == nil || c.Status != models.StatusFail {
		t.Errorf("expected FAIL for missing template ID, got %+v", c)
	}
}

func TestAnalyze_MissingDeepLinkParamsFails(t *testing.T) {
	r, err := onelink.Analyze(context.Background(), newClient(), "https://example.onelink.me/abc/xyz", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "Deep Link Target")
	if c == nil || c.Status != models.StatusFail {
		t.Errorf("expected FAIL when neither deep_link_value nor af_dp is set, got %+v", c)
	}
}

func TestAnalyze_AFDPSatisfiesDeepLinkTarget(t *testing.T) {
	r, err := onelink.Analyze(context.Background(), newClient(), "https://example.onelink.me/abc/xyz?af_dp=myapp://profile/42", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "Deep Link Target")
	if c == nil || c.Status != models.StatusPass {
		t.Errorf("expected PASS when af_dp is set, got %+v", c)
	}
}

func TestAnalyze_MissingWebFallbackWarns(t *testing.T) {
	r, err := onelink.Analyze(context.Background(), newClient(), "https://example.onelink.me/abc/xyz?deep_link_value=x", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "Desktop Fallback")
	if c == nil || c.Status != models.StatusWarning {
		t.Errorf("expected WARN for missing af_web_dp, got %+v", c)
	}
}

func TestAnalyze_MissingMediaSourceWarns(t *testing.T) {
	r, err := onelink.Analyze(context.Background(), newClient(), "https://example.onelink.me/abc/xyz?deep_link_value=x", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "Attribution")
	if c == nil || c.Status != models.StatusWarning {
		t.Errorf("expected WARN for missing pid, got %+v", c)
	}
}

func TestAnalyze_ForceDeepLinkNotTrueWarns(t *testing.T) {
	r, err := onelink.Analyze(context.Background(), newClient(), "https://example.onelink.me/abc/xyz?deep_link_value=x&af_force_deeplink=false", onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "Force Deep Link")
	if c == nil || c.Status != models.StatusWarning {
		t.Errorf("expected WARN for af_force_deeplink=false, got %+v", c)
	}
}

func TestAnalyze_NoAASAOnOneLinkDomainIsInfoNotFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	link := ts.URL + "/abc/xyz?deep_link_value=x&af_web_dp=https://example.com&pid=google&c=x"
	r, err := onelink.Analyze(context.Background(), newClient(), link, onelink.Options{SkipResolve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "OneLink Domain AASA")
	if c == nil || c.Status != models.StatusInfo {
		t.Errorf("expected INFO for missing AASA on the OneLink domain, got %+v", c)
	}
}

func TestAnalyze_ExistingDomainAASAIsValidated(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	link := ts.URL + "/abc/xyz?deep_link_value=x&af_web_dp=https://example.com&pid=google&c=x"
	r, err := onelink.Analyze(context.Background(), newInsecureClient(), link, onelink.Options{
		SkipResolve: true,
		Want:        aasa.AppIdentity{TeamID: "ABCDE12345", BundleID: "com.example.app"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := findCheck(r.Checks, "OneLink Domain AASA: Expected App ID")
	if c == nil || c.Status != models.StatusPass {
		t.Errorf("expected the OneLink domain's AASA to be validated against Want, got checks: %+v", r.Checks)
	}
}

func TestAnalyze_FollowsRedirectAndChecksFinalDomainAASA(t *testing.T) {
	finalTS := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["/landing/*"]}]}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer finalTS.Close()

	oneLinkTS := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, finalTS.URL+"/landing/42", http.StatusFound)
	}))
	defer oneLinkTS.Close()

	link := oneLinkTS.URL + "/abc/xyz?deep_link_value=x&af_web_dp=https://example.com&pid=google&c=x"
	r, err := onelink.Analyze(context.Background(), newInsecureClient(), link, onelink.Options{
		Want: aasa.AppIdentity{TeamID: "ABCDE12345", BundleID: "com.example.app"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Trace == nil {
		t.Fatal("expected a redirect trace to be recorded")
	}

	c := findCheck(r.Checks, "Final Domain AASA: Path Match")
	if c == nil || c.Status != models.StatusPass {
		t.Errorf("expected the final domain's AASA to cover the landing path, got checks: %+v", r.Checks)
	}
}

func TestAnalyze_InvalidScheme(t *testing.T) {
	_, err := onelink.Analyze(context.Background(), newClient(), "myapp://profile", onelink.Options{SkipResolve: true})
	if err == nil {
		t.Fatal("expected an error for a non-http(s) scheme")
	}
}

func TestAnalyze_InvalidURL(t *testing.T) {
	_, err := onelink.Analyze(context.Background(), newClient(), "::not-a-url", onelink.Options{SkipResolve: true})
	if err == nil {
		t.Fatal("expected an error for an invalid URL")
	}
}
