package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/space-code/linkctl/internal/config"
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

func TestLoad_ValidMinimalConfig(t *testing.T) {
	path := writeConfig(t, `
links:
  - url: https://example.com/profile/42
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Links) != 1 || cfg.Links[0].URL != "https://example.com/profile/42" {
		t.Errorf("unexpected links: %+v", cfg.Links)
	}
	if cfg.Links[0].Expect != config.ExpectAuto {
		t.Errorf("expected default Expect to be auto, got %q", cfg.Links[0].Expect)
	}
}

func TestLoad_FullConfig(t *testing.T) {
	path := writeConfig(t, `
project: ./MyApp.xcodeproj
target: MyApp
bundleId: com.example.app
teamId: ABCDE12345
userAgent: ios
timeout: 15s
links:
  - url: https://example.com/profile/42
    expect: universal-link
  - url: https://myapp.onelink.me/abc1/xyz789
    expect: onelink
    finalHost: example.com
  - url: myapp://profile/42
    expect: custom-scheme
checks:
  aasa: true
  appleCdn: false
  appProject: true
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "./MyApp.xcodeproj" || cfg.BundleID != "com.example.app" || cfg.TeamID != "ABCDE12345" {
		t.Errorf("unexpected top-level config: %+v", cfg)
	}
	if len(cfg.Links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(cfg.Links))
	}
	if cfg.Links[1].FinalHost != "example.com" {
		t.Errorf("expected finalHost on the onelink entry, got %q", cfg.Links[1].FinalHost)
	}
	if !cfg.Checks.AASAEnabled() {
		t.Error("expected AASA checks enabled")
	}
	if cfg.Checks.AppleCDNEnabled() {
		t.Error("expected Apple CDN check disabled")
	}
	timeout, err := cfg.Timeout(0)
	if err != nil || timeout != 15*time.Second {
		t.Errorf("expected timeout=15s, got %v (err=%v)", timeout, err)
	}
}

func TestLoad_DefaultsWhenChecksOmitted(t *testing.T) {
	path := writeConfig(t, `
links:
  - url: https://example.com
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Checks.AASAEnabled() || !cfg.Checks.AppleCDNEnabled() || !cfg.Checks.AppProjectEnabled() {
		t.Error("expected all checks to default to enabled when 'checks' is omitted")
	}
}

func TestLoad_UnknownFieldRejected(t *testing.T) {
	path := writeConfig(t, `
links:
  - url: https://example.com
typo_field: oops
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected an error for an unknown top-level field")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "does-not-exist.yml"))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestLoad_NoLinks(t *testing.T) {
	path := writeConfig(t, `project: ./MyApp.xcodeproj`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected an error when no links are configured")
	}
}

func TestLoad_LinkMissingURL(t *testing.T) {
	path := writeConfig(t, `
links:
  - expect: universal-link
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected an error for a link with no url")
	}
}

func TestLoad_InvalidExpectValue(t *testing.T) {
	path := writeConfig(t, `
links:
  - url: https://example.com
    expect: bogus
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected an error for an invalid expect value")
	}
}

func TestLoad_FinalHostWithoutOnelinkExpect(t *testing.T) {
	path := writeConfig(t, `
links:
  - url: https://example.com
    expect: universal-link
    finalHost: example.com
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected an error when finalHost is set without expect: onelink")
	}
}

func TestLoad_InvalidTimeout(t *testing.T) {
	path := writeConfig(t, `
timeout: not-a-duration
links:
  - url: https://example.com
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected an error for an invalid timeout value")
	}
}

func TestConfig_TimeoutDefault(t *testing.T) {
	var cfg config.Config
	d, err := cfg.Timeout(30 * time.Second)
	if err != nil || d != 30*time.Second {
		t.Errorf("expected default timeout to be returned, got %v (err=%v)", d, err)
	}
}
