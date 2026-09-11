package aasa_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/aasa"
	"github.com/space-code/linkctl/pkg/cmdutil"
)

func TestAASACmd_MissingArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when argument is missing")
	}
}

func TestAASACmd_TooManyArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{"example.com", "other.com"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing more than 1 argument")
	}
}

func TestAASACmd_InvalidFormat(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{"example.com", "--format", "yaml"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for an invalid --format value")
	}
}

func TestAASACmd_InvalidSource(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{"example.com", "--source", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for an invalid --source value")
	}
}

// aasaHandler serves a well-formed AASA at the well-known path and 404s
// everywhere else, so root/apple-cdn fetches simply fail (which is fine —
// the test target is the host:port itself, not example.com's real CDN).
func aasaHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/apple-app-site-association" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, body)
			return
		}
		http.NotFound(w, r)
	}
}

// AASA must always be served over HTTPS, so aasa.FetchOne/FetchAll always
// build a https:// URL — tests stand up a TLS server (self-signed cert) and
// pass --insecure, rather than a plain httptest.NewServer.
func newAASATLSServer(body string) *httptest.Server {
	return httptest.NewTLSServer(aasaHandler(body))
}

func TestAASACmd_HappyPath_JSON(t *testing.T) {
	ts := newAASATLSServer(`{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["/profile/*"]}]}}`)
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{
		ts.URL + "/profile/42",
		"--source", "well-known",
		"--bundle-id", "com.example.app",
		"--team-id", "ABCDE12345",
		"--format", "json",
		"--insecure",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}
	if decoded["command"] != "aasa" {
		t.Errorf("expected command=aasa, got %+v", decoded)
	}
}

func TestAASACmd_UncoveredPath_ExitsNonZero(t *testing.T) {
	ts := newAASATLSServer(`{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["/profile/*"]}]}}`)
	defer ts.Close()

	f, _ := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{ts.URL + "/not-covered", "--source", "well-known", "--insecure"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for an uncovered path")
	}
	if !errors.Is(err, cmdutil.ErrChecksFailed) {
		t.Errorf("expected ErrChecksFailed, got %v", err)
	}
}

func TestAASACmd_BareDomainSkipsPathMatch(t *testing.T) {
	ts := newAASATLSServer(`{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["/profile/*"]}]}}`)
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "https://")

	f, stdout := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{host, "--source", "well-known", "--format", "json", "--insecure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for a bare-domain check, got: %v\noutput: %s", err, stdout.String())
	}
	if strings.Contains(stdout.String(), "\"Path Match\"") {
		t.Errorf("did not expect a Path Match check for a bare domain, got: %s", stdout.String())
	}
}

func TestAASACmd_TextOutput(t *testing.T) {
	ts := newAASATLSServer(`{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)
	defer ts.Close()

	f, stdout := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{ts.URL, "--source", "well-known", "--insecure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("expected non-empty text output")
	}
}

// Regression test: --source root (or apple-cdn) fetches only that one
// source; the command must not spuriously fail assuming a well-known
// fetch always exists (see internal/aasa.Check's primaryFetch fallback).
func TestAASACmd_SourceRootOnly_Succeeds(t *testing.T) {
	ts := newAASATLSServer(`{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)
	defer ts.Close()

	// Serve the same AASA body at the legacy root path too.
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"applinks":{"apps":[],"details":[{"appID":"ABCDE12345.com.example.app","paths":["*"]}]}}`)
	})

	f, stdout := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{ts.URL, "--source", "root", "--insecure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "AASA Fetch") {
		t.Errorf("expected an AASA Fetch check in output, got: %s", stdout.String())
	}
}

func TestAASACmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := aasa.NewCmdAASA(f)
	cmd.SetArgs([]string{"example.com", "--nope"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
