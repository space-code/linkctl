package resolve_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/space-code/linkctl/internal/httpx"
	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/resolve"
)

func newClient() *http.Client {
	return httpx.New(httpx.Options{FollowRedirects: false, UserAgent: httpx.UserAgentIOS})
}

func TestFollow_SingleHTTPRedirectThenFinal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/end", http.StatusFound)
		default:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "landing page")
		}
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/start", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trace.Hops) != 2 {
		t.Fatalf("expected 2 hops (redirect + final), got %d: %+v", len(trace.Hops), trace.Hops)
	}
	if trace.Hops[0].Kind != resolve.HopKindHTTP {
		t.Errorf("expected first hop to be HTTP redirect, got %q", trace.Hops[0].Kind)
	}
	if trace.FinalStatus != http.StatusOK {
		t.Errorf("expected final status 200, got %d", trace.FinalStatus)
	}
	if trace.Truncated {
		t.Error("did not expect truncation")
	}
}

func TestFollow_MultiHopChain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusMovedPermanently)
		case "/b":
			http.Redirect(w, r, "/c", http.StatusFound)
		case "/c":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/a", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trace.Hops) != 3 {
		t.Fatalf("expected 3 hops, got %d", len(trace.Hops))
	}
	if trace.Final != ts.URL+"/c" {
		t.Errorf("expected final=%s/c, got %s", ts.URL, trace.Final)
	}
}

func TestFollow_RelativeLocationIsResolved(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "final")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/start", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Final != ts.URL+"/final" {
		t.Errorf("expected relative redirect resolved against base, got %q", trace.Final)
	}
}

func TestFollow_MetaRefresh(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<html><head><meta http-equiv="refresh" content="0; url=/landed"></head></html>`)
		case "/landed":
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/start", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Hops[0].Kind != resolve.HopKindMetaRefresh {
		t.Errorf("expected meta-refresh hop, got %q", trace.Hops[0].Kind)
	}
	if trace.Final != ts.URL+"/landed" {
		t.Errorf("expected to land on /landed, got %q", trace.Final)
	}
}

func TestFollow_JSLocationAssign(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<html><script>window.location.href = "/js-landed";</script></html>`)
		case "/js-landed":
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/start", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Hops[0].Kind != resolve.HopKindJS {
		t.Errorf("expected js hop, got %q", trace.Hops[0].Kind)
	}
	if trace.Final != ts.URL+"/js-landed" {
		t.Errorf("expected to land on /js-landed, got %q", trace.Final)
	}
}

func TestFollow_JSLocationReplace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<script>location.replace('/replaced');</script>`)
		case "/replaced":
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/start", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Final != ts.URL+"/replaced" {
		t.Errorf("expected to land on /replaced, got %q", trace.Final)
	}
}

func TestFollow_StopsAtCustomScheme(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "myapp://profile/42")
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/start", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Final != "myapp://profile/42" {
		t.Errorf("expected final=myapp://profile/42, got %q", trace.Final)
	}
	lastHop := trace.Hops[len(trace.Hops)-1]
	if lastHop.Kind != resolve.HopKindCustomScheme {
		t.Errorf("expected last hop to be custom-scheme, got %q", lastHop.Kind)
	}
}

func TestFollow_CycleDetected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "/a", http.StatusFound)
		}
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/a", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.CycleAt == "" {
		t.Error("expected a cycle to be detected")
	}
	if !trace.Truncated {
		t.Error("expected Truncated=true for a cycle")
	}
}

func TestFollow_ExceedsMaxHops(t *testing.T) {
	var hopCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hopCount++
		http.Redirect(w, r, fmt.Sprintf("/hop%d", hopCount), http.StatusFound)
	}))
	defer ts.Close()

	trace, err := resolve.Follow(context.Background(), newClient(), ts.URL+"/hop0", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !trace.Truncated {
		t.Error("expected Truncated=true when max hops is exceeded")
	}
	if len(trace.Hops) != 3 {
		t.Errorf("expected exactly maxHops=3 hops recorded, got %d", len(trace.Hops))
	}
}

func TestFollow_UnreachableHost(t *testing.T) {
	client := httpx.New(httpx.Options{Timeout: 2e9}) // 2s
	trace, err := resolve.Follow(context.Background(), client, "https://this-host-should-not-resolve.invalid/x", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trace.Issues) == 0 {
		t.Error("expected a FAIL issue for an unreachable host")
	}
}

func TestCheck_PassesWithCustomSchemeDestination(t *testing.T) {
	trace := &resolve.Trace{
		Start: "https://example.onelink.me/abc",
		Final: "myapp://profile/42",
		Hops: []resolve.Hop{
			{URL: "https://example.onelink.me/abc", Kind: resolve.HopKindHTTP, StatusCode: 302},
			{URL: "myapp://profile/42", Kind: resolve.HopKindCustomScheme},
		},
	}

	results := resolve.Check(trace)

	foundPass := false
	for _, r := range results {
		if r.Check == "Final Destination" && r.Status == models.StatusPass {
			foundPass = true
		}
	}
	if !foundPass {
		t.Errorf("expected a PASS Final Destination check, got %+v", results)
	}
}

func TestCheck_FlagsDeadEnd4xx(t *testing.T) {
	trace := &resolve.Trace{
		Start:       "https://example.com/x",
		Final:       "https://example.com/404",
		FinalStatus: 404,
		Hops: []resolve.Hop{
			{URL: "https://example.com/404", Kind: resolve.HopKindFinal, StatusCode: 404},
		},
	}

	results := resolve.Check(trace)

	foundFail := false
	for _, r := range results {
		if r.Check == "Final Destination" && r.Status == models.StatusFail {
			foundFail = true
		}
	}
	if !foundFail {
		t.Errorf("expected a FAIL Final Destination check for a 404 dead end, got %+v", results)
	}
}

func TestCheck_PropagatesCycleFailure(t *testing.T) {
	trace := &resolve.Trace{
		Start:     "https://example.com/a",
		CycleAt:   "https://example.com/a",
		Truncated: true,
		Issues: []models.ValidationResult{
			{Check: "Redirect Chain", Status: models.StatusFail, Message: "redirect loop detected"},
		},
	}

	results := resolve.Check(trace)
	if len(results) == 0 || results[0].Status != models.StatusFail {
		t.Errorf("expected the cycle failure to propagate, got %+v", results)
	}
}
