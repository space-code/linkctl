package validator_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/space-code/linkctl/internal/validator"
)

func TestValidateDeepLink_CustomScheme(t *testing.T) {
	res, err := validator.ValidateDeepLink("myapp://profile/123", validator.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.HasErrors() {
		t.Errorf("expected custom scheme to be valid without errors")
	}
}

func TestValidateDeepLink_InvalidURL(t *testing.T) {
	_, err := validator.ValidateDeepLink("::not-a-url", validator.Options{})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// AASA must be served over HTTPS, so validator.ValidateDeepLink always
// fetches via https:// — tests use a TLS server (self-signed cert) with
// Insecure: true, rather than a plain httptest.NewServer.
func TestValidateDeepLink_MockServer(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/apple-app-site-association":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"applinks": {"apps": [], "details": [{"appID": "ABCDE12345.com.example.app", "paths": ["*"]}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	res, err := validator.ValidateDeepLink(server.URL+"/test", validator.Options{Insecure: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.HasErrors() {
		t.Errorf("expected validation to pass, got issues: %+v", res.Issues)
	}
}

func TestValidateDeepLink_RejectsInvalidTLSByDefault(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	res, err := validator.ValidateDeepLink(server.URL+"/test", validator.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.HasErrors() {
		t.Error("expected a TLS verification error for a self-signed certificate without --insecure")
	}
}

func TestValidateDeepLink_UncoveredPathFails(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"applinks": {"apps": [], "details": [{"appID": "ABCDE12345.com.example.app", "paths": ["/profile/*"]}]}}`)
	}))
	defer server.Close()

	res, err := validator.ValidateDeepLink(server.URL+"/settings", validator.Options{Insecure: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.HasErrors() {
		t.Error("expected an error for a path not covered by applinks.details")
	}
}

func TestValidateDeepLink_CoveredPathPasses(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"applinks": {"apps": [], "details": [{"appID": "ABCDE12345.com.example.app", "paths": ["/profile/*"]}]}}`)
	}))
	defer server.Close()

	res, err := validator.ValidateDeepLink(server.URL+"/profile/42", validator.Options{Insecure: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HasErrors() {
		t.Errorf("expected a covered path to pass, got issues: %+v", res.Issues)
	}
}
