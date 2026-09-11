package cache_test

import (
	"testing"

	"github.com/space-code/linkctl/internal/testutil"
	"github.com/space-code/linkctl/pkg/cmd/cache"
)

func TestCacheResetCmd_MissingRequiredPlatformFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := cache.NewCmdCacheReset(f)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --platform flag is missing")
	}
}

func TestCacheResetCmd_UnknownFlag(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := cache.NewCmdCacheReset(f)
	cmd.SetArgs([]string{"--platform", "ios", "--unknown-flag"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestCacheResetCmd_InvalidPlatform(t *testing.T) {
	f, _ := testutil.NewFactory(t)
	cmd := cache.NewCmdCacheReset(f)
	cmd.SetArgs([]string{"--platform", "windows"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}
