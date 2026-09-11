package onelink_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/onelink"
	"github.com/space-code/linkctl/pkg/cmdutil"
)

func TestOneLinkCmd_MissingArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when link argument is missing")
	}
}

func TestOneLinkCmd_TooManyArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{"https://a.onelink.me/1", "https://b.onelink.me/2"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing more than 1 argument")
	}
}

func TestOneLinkCmd_InvalidFormat(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{"https://example.onelink.me/abc/xyz", "--format", "bogus", "--no-resolve"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid --format")
	}
}

func TestOneLinkCmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{"https://example.onelink.me/abc/xyz", "--nope"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestOneLinkCmd_HappyPath_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	link := ts.URL + "/abc/xyz?deep_link_value=x&af_web_dp=https://example.com&pid=google&c=summer"

	f, stdout := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{link, "--no-resolve", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}
	if decoded["command"] != "onelink" {
		t.Errorf("expected command=onelink, got %+v", decoded)
	}
}

func TestOneLinkCmd_MissingDeepLinkParams_ExitsNonZero(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	f, _ := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{ts.URL + "/abc/xyz", "--no-resolve"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when no deep_link_value/af_dp param is present")
	}
	if !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed, got %v", err)
	}
}

func TestOneLinkCmd_TextOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	link := ts.URL + "/abc/xyz?deep_link_value=x&af_web_dp=https://example.com&pid=google&c=summer"

	f, stdout := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{link, "--no-resolve"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("expected non-empty text output")
	}
}

func TestOneLinkCmd_ExpectedAppIDFromFlags(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	link := ts.URL + "/abc/xyz?deep_link_value=x&af_web_dp=https://example.com&pid=google&c=summer"

	f, stdout := testutil.NewFactory(t)
	cmd := onelink.NewCmdOneLink(f)
	cmd.SetArgs([]string{
		link, "--no-resolve", "--insecure",
		"--bundle-id", "com.example.app", "--team-id", "ABCDE12345",
		"--format", "json",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Expected App ID") {
		t.Errorf("expected an 'Expected App ID' check in output, got: %s", stdout.String())
	}
}
