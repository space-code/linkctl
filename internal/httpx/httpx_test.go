package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/space-code/linkctl/internal/httpx"
)

func TestNew_SendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpx.New(httpx.Options{UserAgent: httpx.UserAgentIOS})
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if gotUA != httpx.UserAgentIOS {
		t.Errorf("expected User-Agent %q, got %q", httpx.UserAgentIOS, gotUA)
	}
}

func TestNew_DoesNotFollowRedirectsByDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpx.New(httpx.Options{FollowRedirects: false})
	resp, err := client.Get(ts.URL + "/start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected redirect to be surfaced as 302, got %d", resp.StatusCode)
	}
}

func TestNew_FollowsRedirectsWhenEnabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpx.New(httpx.Options{FollowRedirects: true})
	resp, err := client.Get(ts.URL + "/start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected the client to follow the redirect to 200, got %d", resp.StatusCode)
	}
}

func TestNew_RejectsInvalidTLSByDefault(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpx.New(httpx.Options{Insecure: false})
	_, err := client.Get(ts.URL)
	if err == nil {
		t.Fatal("expected a TLS verification error for a self-signed certificate, got nil")
	}
}

func TestNew_InsecureAcceptsSelfSignedCert(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpx.New(httpx.Options{Insecure: true})
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error with Insecure=true: %v", err)
	}
	defer resp.Body.Close()
}

func TestNew_DefaultTimeoutApplied(t *testing.T) {
	client := httpx.New(httpx.Options{})
	if client.Timeout != httpx.DefaultTimeout {
		t.Errorf("expected default timeout %v, got %v", httpx.DefaultTimeout, client.Timeout)
	}
}

func TestNew_CustomTimeoutApplied(t *testing.T) {
	client := httpx.New(httpx.Options{Timeout: 3 * time.Second})
	if client.Timeout != 3*time.Second {
		t.Errorf("expected custom timeout, got %v", client.Timeout)
	}
}

func TestResolveUserAgent(t *testing.T) {
	tests := []struct {
		preset string
		want   string
	}{
		{"", httpx.UserAgentIOS},
		{"ios", httpx.UserAgentIOS},
		{"android", httpx.UserAgentAndroid},
		{"desktop", httpx.UserAgentDesktop},
		{"bot", httpx.UserAgentBot},
		{"custom-ua-string", "custom-ua-string"},
	}
	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			got := httpx.ResolveUserAgent(tt.preset)
			if got != tt.want {
				t.Errorf("ResolveUserAgent(%q) = %q, want %q", tt.preset, got, tt.want)
			}
		})
	}
}
