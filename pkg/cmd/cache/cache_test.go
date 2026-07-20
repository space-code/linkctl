package cache_test

import (
	"bytes"
	"testing"

	"github.com/space-code/linkctl/pkg/cmd/cache"
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

func TestCacheResetCmd_MissingRequiredPlatformFlag(t *testing.T) {
	f, _ := newFactory(t)
	cmd := cache.NewCmdCacheReset(f)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --platform flag is missing")
	}
}

func TestCacheResetCmd_UnknownFlag(t *testing.T) {
	f, _ := newFactory(t)
	cmd := cache.NewCmdCacheReset(f)
	cmd.SetArgs([]string{"--platform", "ios", "--unknown-flag"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestCacheResetCmd_InvalidPlatform(t *testing.T) {
	f, _ := newFactory(t)
	cmd := cache.NewCmdCacheReset(f)
	cmd.SetArgs([]string{"--platform", "windows"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}
