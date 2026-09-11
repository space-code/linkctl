package validate_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/validate"
	"github.com/space-code/linkctl/pkg/cmdutil"
)

// AASA must be served over HTTPS, so `validate` always fetches via
// https:// — the mock server is TLS (self-signed cert), and tests pass
// --insecure to accept it.
func createMockValidationServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/apple-app-site-association":
			fmt.Fprintln(w, `{"applinks": {"apps": [], "details": [{"appID": "ABCDE12345.com.example.app", "paths": ["*"]}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestValidateCmd_MissingArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when link argument is missing")
	}
}

func TestValidateCmd_TooManyArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{"https://example.com/1", "https://example.com/2"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing more than 1 argument")
	}
}

func TestValidateCmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{"https://example.com", "--invalid-flag"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestValidateCmd_Execution_JSON(t *testing.T) {
	ts := createMockValidationServer()
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := validate.NewCmdValidate(f)

	targetURL := ts.URL + "/profile"
	cmd.SetArgs([]string{targetURL, "--json", "--insecure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error during execution: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}

	valid, ok := result["valid"].(bool)
	if !ok {
		t.Fatalf("expected 'valid' boolean field in output, got: %v", result)
	}

	if !valid {
		t.Errorf("expected validation to succeed for mock server, got valid=false")
	}
}

func TestValidateCmd_Execution_TextOutput(t *testing.T) {
	ts := createMockValidationServer()
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := validate.NewCmdValidate(f)

	targetURL := ts.URL + "/profile"
	cmd.SetArgs([]string{targetURL, "--insecure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error during execution: %v", err)
	}

	if stdout.Len() == 0 {
		t.Error("expected text reporter output, got empty stdout")
	}
}

func TestValidateCmd_UncoveredPath_ExitsNonZero(t *testing.T) {
	// This mock server restricts paths (unlike createMockValidationServer's
	// "*"), so a path outside /profile/* is genuinely uncovered.
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"applinks": {"apps": [], "details": [{"appID": "ABCDE12345.com.example.app", "paths": ["/profile/*"]}]}}`)
	}))
	defer ts.Close()

	f, _ := testutil.NewFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{ts.URL + "/settings", "--insecure"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a path not covered by applinks.details")
	}
	if !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed, got %v", err)
	}
}

func TestValidateCmd_WithoutInsecure_FailsOnSelfSignedCert(t *testing.T) {
	ts := createMockValidationServer()
	defer ts.Close()

	f, _ := testutil.NewFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{ts.URL + "/profile"}) // no --insecure

	err := cmd.Execute()
	if !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed for an unverified self-signed cert, got %v", err)
	}
}
