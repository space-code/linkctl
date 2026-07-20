package validator_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/space-code/linkctl/internal/validator"
)

func TestValidateDeepLink_CustomScheme(t *testing.T) {
	res, err := validator.ValidateDeepLink("myapp://profile/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.HasErrors() {
		t.Errorf("expected custom scheme to be valid without errors")
	}
}

func TestValidateDeepLink_InvalidURL(t *testing.T) {
	_, err := validator.ValidateDeepLink("::not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestValidateDeepLink_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/apple-app-site-association":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"applinks": {"details": []}}`)
		case "/.well-known/assetlinks.json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `[{"relation": ["delegate_permission/common.handle_all_urls"]}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	res, err := validator.ValidateDeepLink(server.URL + "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.HasErrors() {
		t.Errorf("expected validation to pass, got issues: %+v", res.Issues)
	}
}
