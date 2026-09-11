package ci_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/ci"
	"github.com/space-code/linkctl/pkg/cmdutil"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "linkctl.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestCICmd_TakesNoArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"unexpected-arg"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing a positional argument")
	}
}

func TestCICmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--nope"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestCICmd_InvalidFormat(t *testing.T) {
	path := writeConfig(t, "links:\n  - url: myapp://x\n")
	f, _ := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path, "--format", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid --format")
	}
}

func TestCICmd_InvalidFailOn(t *testing.T) {
	path := writeConfig(t, "links:\n  - url: myapp://x\n")
	f, _ := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path, "--fail-on", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid --fail-on")
	}
}

func TestCICmd_MissingConfigFile(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "nope.yml")})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for a missing config file")
	}
}

func TestCICmd_CustomSchemeOnly_Passes(t *testing.T) {
	path := writeConfig(t, "links:\n  - url: myapp://profile/42\n    expect: custom-scheme\n")

	f, stdout := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path, "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}
	if decoded["command"] != "ci" {
		t.Errorf("expected command=ci, got %+v", decoded)
	}
}

func TestCICmd_UniversalLinkAutoInferred_PassesWithGoodAASA(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	path := writeConfig(t, fmt.Sprintf(`
bundleId: com.example.app
teamId: ABCDE12345
links:
  - url: %s/profile/42
checks:
  appleCdn: false
`, ts.URL))

	f, stdout := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path, "--format", "json", "--insecure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}
}

func TestCICmd_FailingLink_ExitsNonZero(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	path := writeConfig(t, fmt.Sprintf("links:\n  - url: %s/profile/42\nchecks:\n  appleCdn: false\n", ts.URL))

	f, _ := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path, "--insecure"})

	err := cmd.Execute()
	if !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed for a 404 AASA endpoint, got %v", err)
	}
}

func TestCICmd_MultipleLinks_OneSectionEach(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	path := writeConfig(t, fmt.Sprintf(`
links:
  - url: %s/a
  - url: myapp://custom
    expect: custom-scheme
checks:
  appleCdn: false
`, ts.URL))

	f, stdout := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path, "--format", "json", "--insecure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}

	var decoded struct {
		Sections []struct {
			Name string `json:"name"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(decoded.Sections) != 2 {
		t.Fatalf("expected 2 sections (one per link), got %d", len(decoded.Sections))
	}
}

func TestCICmd_FailOnWarning(t *testing.T) {
	// A domain with no AASA at all is only a WARN under --source-style
	// checks elsewhere, but a genuinely malformed detail entry produces a
	// WARN here too (missing components/paths is a warning, not a failure).
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app"}]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	path := writeConfig(t, fmt.Sprintf("links:\n  - url: %s\nchecks:\n  appleCdn: false\n", ts.URL))

	f, _ := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path, "--insecure"}) // default --fail-on error

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success with default --fail-on=error (only a WARN present), got %v", err)
	}

	cmd2 := ci.NewCmdCI(f)
	cmd2.SetArgs([]string{"--config", path, "--fail-on", "warning", "--insecure"})
	if err := cmd2.Execute(); !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed with --fail-on=warning, got %v", err)
	}
}

func TestCICmd_TextOutput(t *testing.T) {
	path := writeConfig(t, "links:\n  - url: myapp://x\n    expect: custom-scheme\n")

	f, stdout := testutil.NewFactory(t)
	cmd := ci.NewCmdCI(f)
	cmd.SetArgs([]string{"--config", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("expected non-empty text output")
	}
}
