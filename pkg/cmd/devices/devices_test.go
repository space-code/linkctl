package devices_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/space-code/linkctl/pkg/cmd/devices"
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

func TestDevicesCmd_NoError(t *testing.T) {
	f, _ := newFactory(t)
	cmd := devices.NewCmdDevices(f)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevicesCmd_JSONOutput_HasAllKeys(t *testing.T) {
	f, stdout := newFactory(t)
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
	f, _ := newFactory(t)
	cmd := devices.NewCmdDevices(f)
	cmd.SetArgs([]string{"--unknown"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("unexpected error for unknown flag")
	}
}

func TestDevicesCmd_JSONOutput_Shape(t *testing.T) {
	f, stdout := newFactory(t)
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
