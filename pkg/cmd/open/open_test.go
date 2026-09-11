package open_test

import (
	"encoding/json"
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/open"
)

func TestOpenCmd_MissingArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := open.NewCmdOpen(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when link argument is missing")
	}
}

func TestOpenCmd_TooManyArgs(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := open.NewCmdOpen(f)
	cmd.SetArgs([]string{"myapp://a", "myapp://b"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing more than 1 argument")
	}
}

func TestOpenCmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := open.NewCmdOpen(f)
	cmd.SetArgs([]string{"myapp://profile", "--nope"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestOpenCmd_InvalidLink(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := open.NewCmdOpen(f)
	cmd.SetArgs([]string{"   "})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for a blank link")
	}
}

// No CI runner has a booted simulator by default, and a distinctive fake
// custom scheme guarantees no real app would claim it even if one were —
// so `xcrun simctl openurl` reliably fails here, exercising the failure
// path (ErrChecksFailed + populated Output/Error) without needing a mock.
func TestOpenCmd_NoBootedSimulator_ExitsNonZero(t *testing.T) {
	f, stdout := testutil.NewFactory(t)
	cmd := open.NewCmdOpen(f)
	cmd.SetArgs([]string{"linkctl-test-scheme-zzz://probe", "--json"})

	err := cmd.Execute()
	if err == nil {
		t.Skip("a simulator appears to be booted and accepted the link in this environment")
	}

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}
	if result.Success {
		t.Error("expected success=false in the JSON output")
	}
	if result.Error == "" {
		t.Error("expected a non-empty error message in the JSON output")
	}
}

func TestOpenCmd_TextOutputOnFailure(t *testing.T) {
	f, stdout := testutil.NewFactory(t)
	cmd := open.NewCmdOpen(f)
	cmd.SetArgs([]string{"linkctl-test-scheme-zzz://probe"})

	if err := cmd.Execute(); err == nil {
		t.Skip("a simulator appears to be booted and accepted the link in this environment")
	}
	if stdout.Len() == 0 {
		t.Error("expected non-empty text output even on failure")
	}
}
