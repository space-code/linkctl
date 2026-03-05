package scan_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/space-code/linkctl/pkg/cmd/scan"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/space-code/linkctl/pkg/iostreams"
)

const mockPbxproj = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>archiveVersion</key>
    <string>1</string>
    <key>classes</key>
    <dict/>
    <key>objectVersion</key>
    <string>46</string>
    <key>objects</key>
    <dict>
        <key>PROJECT_ID</key>
        <dict>
            <key>isa</key>
            <string>PBXProject</string>
            <key>buildConfigurationList</key>
            <string>PROJECT_CONFIG_LIST</string>
            <key>targets</key>
            <array>
                <string>TARGET_ID</string>
            </array>
            <key>attributes</key>
            <dict/>
        </dict>
        <key>PROJECT_CONFIG_LIST</key>
        <dict>
            <key>isa</key>
            <string>XCConfigurationList</string>
            <key>buildConfigurations</key>
            <array>
                <string>PROJECT_CONFIG_ID_RELEASE</string>
            </array>
            <key>defaultConfigurationName</key>
            <string>Release</string>
        </dict>
        <key>PROJECT_CONFIG_ID_RELEASE</key>
        <dict>
            <key>isa</key>
            <string>XCBuildConfiguration</string>
            <key>name</key>
            <string>Release</string>
            <key>buildSettings</key>
            <dict/>
        </dict>
        <key>TARGET_ID</key>
        <dict>
            <key>isa</key>
            <string>PBXNativeTarget</string>
            <key>name</key>
            <string>MyApp</string>
            <key>productType</key>
            <string>com.apple.product-type.application</string>
            <key>buildConfigurationList</key>
            <string>TARGET_CONFIG_LIST</string>
            <key>dependencies</key>
            <array/>
            <key>buildPhases</key>
            <array/>
        </dict>
        <key>TARGET_CONFIG_LIST</key>
        <dict>
            <key>isa</key>
            <string>XCConfigurationList</string>
            <key>buildConfigurations</key>
            <array>
                <string>TARGET_CONFIG_ID_RELEASE</string>
            </array>
            <key>defaultConfigurationName</key>
            <string>Release</string>
        </dict>
        <key>TARGET_CONFIG_ID_RELEASE</key>
        <dict>
            <key>isa</key>
            <string>XCBuildConfiguration</string>
            <key>name</key>
            <string>Release</string>
            <key>buildSettings</key>
            <dict>
                <key>PRODUCT_BUNDLE_IDENTIFIER</key>
                <string>com.example.myapp</string>
                <key>CODE_SIGN_ENTITLEMENTS</key>
                <string>MyApp.entitlements</string>
                <key>INFOPLIST_FILE</key>
                <string>Info.plist</string>
            </dict>
        </dict>
    </dict>
    <key>rootObject</key>
    <string>PROJECT_ID</string>
</dict>
</plist>`

const mockEntitlements = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.developer.associated-domains</key>
    <array>
        <string>applinks:example.com</string>
    </array>
</dict>
</plist>`

const mockInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>com.example.myapp</string>
    <key>CFBundleURLTypes</key>
    <array>
        <dict>
            <key>CFBundleURLName</key>
            <string>myapp-scheme</string>
            <key>CFBundleURLSchemes</key>
            <array>
                <string>myapp</string>
            </array>
        </dict>
    </array>
</dict>
</plist>`

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

func createMockIOSProject(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "linkctl-scan-ios-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	xcodeprojPath := filepath.Join(tmpDir, "Mock.xcodeproj")
	if err := os.MkdirAll(xcodeprojPath, 0o755); err != nil {
		t.Fatalf("failed to create xcodeproj dir: %v", err)
	}

	err = os.WriteFile(filepath.Join(xcodeprojPath, "project.pbxproj"), []byte(mockPbxproj), 0o644)
	if err != nil {
		t.Fatalf("failed to write project.pbxproj: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "MyApp.entitlements"), []byte(mockEntitlements), 0o644)
	if err != nil {
		t.Fatalf("failed to write MyApp.entitlements: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "Info.plist"), []byte(mockInfoPlist), 0o644)
	if err != nil {
		t.Fatalf("failed to write Info.plist: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestScanCmd_TooManyArgs(t *testing.T) {
	f, _ := newFactory(t)
	cmd := scan.NewCmdScan(f)
	cmd.SetArgs([]string{"./path1", "./path2"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when passing more than 1 argument")
	}
}

func TestScanCmd_UnknownFlag(t *testing.T) {
	f, _ := newFactory(t)
	cmd := scan.NewCmdScan(f)
	cmd.SetArgs([]string{"--invalid-flag"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestScanCmd_NonExistentDirectory(t *testing.T) {
	f, _ := newFactory(t)
	cmd := scan.NewCmdScan(f)
	cmd.SetArgs([]string{"/non/existent/path/for/linkctl/test"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for non-existent directory scan")
	}
}

func TestScanCmd_IOSProject_JSON(t *testing.T) {
	f, stdout := newFactory(t)
	projDir, cleanup := createMockIOSProject(t)
	defer cleanup()

	cmd := scan.NewCmdScan(f)
	cmd.SetArgs([]string{projDir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error during scan execution: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}

	links, ok := result["registered_links"].([]interface{})
	if !ok {
		t.Fatalf("expected 'registered_links' array in output, got: %v", result)
	}

	if len(links) == 0 {
		t.Errorf("expected to find registered links in mock iOS project, found 0")
	}
}

func TestScanCmd_IOSProject_TextOutput(t *testing.T) {
	f, stdout := newFactory(t)
	projDir, cleanup := createMockIOSProject(t)
	defer cleanup()

	cmd := scan.NewCmdScan(f)
	cmd.SetArgs([]string{projDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error during scan execution: %v", err)
	}

	if stdout.Len() == 0 {
		t.Error("expected text reporter output, got empty stdout")
	}
}
