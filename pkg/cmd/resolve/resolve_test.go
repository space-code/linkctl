package resolve_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/resolve"
	"github.com/space-code/linkctl/pkg/cmdutil"
)

func TestResolveCmd_MissingArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when link argument is missing")
	}
}

func TestResolveCmd_TooManyArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{"https://a.com", "https://b.com"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing more than 1 argument")
	}
}

func TestResolveCmd_InvalidFormat(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{"https://example.com", "--format", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid --format")
	}
}

func TestResolveCmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{"https://example.com", "--nope"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestResolveCmd_HappyPath_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{ts.URL + "/start", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}
	if decoded["command"] != "resolve" {
		t.Errorf("expected command=resolve, got %+v", decoded)
	}
}

func TestResolveCmd_CustomSchemeDestination_Succeeds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "myapp://profile/42")
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{ts.URL, "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}
}

func TestResolveCmd_RedirectLoop_ExitsNonZero(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "/a", http.StatusFound)
		}
	}))
	defer ts.Close()

	f, _ := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{ts.URL + "/a"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a redirect loop")
	}
	if !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed, got %v", err)
	}
}

func TestResolveCmd_DeadEnd404_ExitsNonZero(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	f, _ := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{ts.URL})

	err := cmd.Execute()
	if !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed for a 404 dead end, got %v", err)
	}
}

// A chain that exceeds --max-hops is inconclusive, not a definite failure
// (the link might still resolve fine one hop later) — it's reported as a
// WARN and does not fail the command on its own.
func TestResolveCmd_MaxHopsFlag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{ts.URL, "--max-hops", "2", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}

	var decoded struct {
		Sections []struct {
			Checks []struct {
				Check  string `json:"check"`
				Status string `json:"status"`
			} `json:"checks"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	var sawWarn bool
	for _, c := range decoded.Sections[0].Checks {
		if c.Status == "WARN" {
			sawWarn = true
		}
	}
	if !sawWarn {
		t.Errorf("expected a WARN check for a truncated chain, got %+v", decoded.Sections[0].Checks)
	}
}

func TestResolveCmd_TextOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := resolve.NewCmdResolve(f)
	cmd.SetArgs([]string{ts.URL})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("expected non-empty text output")
	}
}
