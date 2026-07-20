package validate_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/space-code/linkctl/pkg/cmd/validate"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/space-code/linkctl/pkg/iostreams"
)

func newFactory(t *testing.T) (*cmdutil.Factory, *bytes.Buffer) {
	t.Helper()

	ios, _, stdout, _ := iostreams.Test()

	f := &cmdutil.Factory{
		AppVersion:     "1.0.0",
		ExecutableName: "linkctl",
		IOStreams:      ios,
	}

	return f, stdout
}

func createMockValidationServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/apple-app-site-association":
			fmt.Fprintln(w, `{"applinks": {"details": []}}`)
		case "/.well-known/assetlinks.json":
			fmt.Fprintln(w, `[{"relation": ["delegate_permission/common.handle_all_urls"]}]`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestValidateCmd_MissingArgs(t *testing.T) {
	f, _ := newFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when link argument is missing")
	}
}

func TestValidateCmd_TooManyArgs(t *testing.T) {
	f, _ := newFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{"https://example.com/1", "https://example.com/2"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing more than 1 argument")
	}
}

func TestValidateCmd_UnknownFlag(t *testing.T) {
	f, _ := newFactory(t)
	cmd := validate.NewCmdValidate(f)
	cmd.SetArgs([]string{"https://example.com", "--invalid-flag"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestValidateCmd_Execution_JSON(t *testing.T) {
	ts := createMockValidationServer()
	defer ts.Close()

	f, stdout := newFactory(t)
	cmd := validate.NewCmdValidate(f)

	targetURL := ts.URL + "/profile"
	cmd.SetArgs([]string{targetURL, "--json"})

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

	f, stdout := newFactory(t)
	cmd := validate.NewCmdValidate(f)

	targetURL := ts.URL + "/profile"
	cmd.SetArgs([]string{targetURL})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error during execution: %v", err)
	}

	if stdout.Len() == 0 {
		t.Error("expected text reporter output, got empty stdout")
	}
}
