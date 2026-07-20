package check_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/space-code/linkctl/pkg/cmd/check"
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
				<string>PROJECT_CONFIG_ID_DEBUG</string>
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
		<key>PROJECT_CONFIG_ID_DEBUG</key>
		<dict>
			<key>isa</key>
			<string>XCBuildConfiguration</string>
			<key>name</key>
			<string>Debug</string>
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

func createMockProject(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "linkctl-cmd-test-*")
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

func TestCheckCmd_InvalidLink(t *testing.T) {
	f, _ := newFactory(t)
	cmd := check.NewCmdCheck(f)
	cmd.SetArgs([]string{""})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for empty link")
	}
}

func TestCheckCmd_AndroidProject(t *testing.T) {
	f, stdout := newFactory(t)

	// Create dummy android project (just Manifest)
	tmpDir, err := os.MkdirTemp("", "linkctl-android-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(filepath.Join(tmpDir, "AndroidManifest.xml"), []byte("<manifest></manifest>"), 0o644)
	if err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	cmd := check.NewCmdCheck(f)
	cmd.SetArgs([]string{"https://example.com/profile", "--project", tmpDir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}

	if result["platform"] != "android" {
		t.Errorf("expected platform 'android', got '%v'", result["platform"])
	}
}

func TestCheckCmd_IOSProject_WithConfiguration(t *testing.T) {
	f, stdout := newFactory(t)
	projDir, cleanup := createMockProject(t)
	defer cleanup()

	cmd := check.NewCmdCheck(f)
	// Check deep link check-app against iOS project, passing configuration explicitly
	cmd.SetArgs([]string{"https://example.com/profile", "--project", projDir, "--configuration", "Release", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, stdout.String())
	}

	if result["platform"] != "ios" {
		t.Errorf("expected platform 'ios', got '%v'", result["platform"])
	}

	// Verify deep link checks summary
	summary, ok := result["summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("summary key not found or not a map in JSON output")
	}

	if summary["ok"] != true {
		t.Errorf("expected summary.ok to be true, got %v. JSON: %s", summary["ok"], stdout.String())
	}
}

func TestCheckCmd_UnknownFlag(t *testing.T) {
	f, _ := newFactory(t)
	cmd := check.NewCmdCheck(f)
	cmd.SetArgs([]string{"https://example.com", "--unknown"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
