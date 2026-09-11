package aasa_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/httpx"
)

func newTestClient() *http.Client {
	return httpx.New(httpx.Options{FollowRedirects: false, UserAgent: httpx.UserAgentIOS})
}

// domainFromServer strips the scheme from an httptest server URL so it can
// be passed to FetchAll/FetchOne, which always prepend "https://" — tests
// instead redirect that host to the httptest server via a custom transport.
type schemeRewriteTransport struct {
	target string // e.g. "127.0.0.1:54321"
}

func (t *schemeRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = t.target
	req.Host = t.target
	return http.DefaultTransport.RoundTrip(req)
}

func clientForServer(ts *httptest.Server) *http.Client {
	host := strings.TrimPrefix(strings.TrimPrefix(ts.URL, "http://"), "https://")
	return &http.Client{
		Transport: &schemeRewriteTransport{target: host},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestFetchOne_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"applinks":{"apps":[],"details":[]}}`))
	}))
	defer ts.Close()

	f := aasa.FetchOne(context.Background(), clientForServer(ts), "example.com", aasa.SourceWellKnown)

	if f.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", f.StatusCode)
	}
	if f.Redirected {
		t.Error("did not expect a redirect")
	}
	if f.SizeBytes == 0 {
		t.Error("expected a non-zero body size")
	}
	if f.Source != aasa.SourceWellKnown {
		t.Errorf("expected Source=well-known, got %q", f.Source)
	}
}

func TestFetchOne_RedirectIsRecordedNotFollowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/moved", http.StatusMovedPermanently)
	}))
	defer ts.Close()

	f := aasa.FetchOne(context.Background(), clientForServer(ts), "example.com", aasa.SourceWellKnown)

	if !f.Redirected {
		t.Error("expected Redirected=true")
	}
	if f.Location != "/moved" {
		t.Errorf("expected Location header to be captured, got %q", f.Location)
	}
}

func TestFetchOne_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	f := aasa.FetchOne(context.Background(), clientForServer(ts), "example.com", aasa.SourceWellKnown)

	if f.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", f.StatusCode)
	}
	if f.Body != nil {
		t.Error("expected no body to be captured for a non-200 response")
	}
}

func TestFetchOne_BodyTooLarge(t *testing.T) {
	oversized := strings.Repeat("a", aasa.MaxSizeBytes+1024)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"pad":"` + oversized + `"}`))
	}))
	defer ts.Close()

	f := aasa.FetchOne(context.Background(), clientForServer(ts), "example.com", aasa.SourceWellKnown)

	if f.SizeBytes <= aasa.MaxSizeBytes {
		t.Errorf("expected the recorded size to exceed the limit, got %d", f.SizeBytes)
	}
}

func TestFetchOne_UnreachableHost(t *testing.T) {
	client := httpx.New(httpx.Options{Timeout: 2 * time.Second})
	f := aasa.FetchOne(context.Background(), client, "this-host-should-not-resolve.invalid", aasa.SourceWellKnown)

	if f.FetchError == "" {
		t.Error("expected a fetch error for an unresolvable host")
	}
}

func TestFetchAll_QueriesAllThreeSources(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"applinks":{"apps":[],"details":[]}}`))
	}))
	defer ts.Close()

	fetches := aasa.FetchAll(context.Background(), clientForServer(ts), "example.com")

	if len(fetches) != 3 {
		t.Fatalf("expected 3 fetches (well-known, root, apple-cdn), got %d", len(fetches))
	}

	seen := map[aasa.Source]bool{}
	for _, f := range fetches {
		seen[f.Source] = true
	}
	for _, want := range []aasa.Source{aasa.SourceWellKnown, aasa.SourceRoot, aasa.SourceAppleCDN} {
		if !seen[want] {
			t.Errorf("expected a fetch for source %q", want)
		}
	}
}

func TestFetchOne_UnknownSource(t *testing.T) {
	client := newTestClient()
	f := aasa.FetchOne(context.Background(), client, "example.com", aasa.Source("bogus"))
	if f.FetchError == "" {
		t.Error("expected an error for an unknown source")
	}
}
