package devices_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/devices"
)

func TestDevicesCmd_NoError(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := devices.NewCmdDevices(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevicesCmd_JSONOutput_HasAllKeys(t *testing.T) {
	f, stdout := testutil.NewFactory(t)
	cmd := devices.NewCmdDevices(f)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := stdout.String()
	for _, key := range []string{"ios", "tools"} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON output missing key %s\ngot: %s", key, raw)
		}
	}
}

func TestDevicesCmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := devices.NewCmdDevices(f)
	cmd.SetArgs([]string{"--unknown"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("unexpected error for unknown flag")
	}
}

func TestDevicesCmd_JSONOutput_Shape(t *testing.T) {
	f, stdout := testutil.NewFactory(t)
	cmd := devices.NewCmdDevices(f)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload struct {
		IOS   []string        `json:"ios"`
		Tools map[string]bool `json:"tools"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}

	for _, key := range []string{"xcrun"} {
		if _, ok := payload.Tools[key]; !ok {
			t.Errorf("uexpected tools map to contain key %q", key)
		}
	}
}
