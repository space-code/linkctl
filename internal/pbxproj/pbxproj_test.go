package pbxproj_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/space-code/linkctl/internal/pbxproj"
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
				<string>TARGET_CONFIG_ID_DEBUG</string>
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
		<key>TARGET_CONFIG_ID_DEBUG</key>
		<dict>
			<key>isa</key>
			<string>XCBuildConfiguration</string>
			<key>name</key>
			<string>Debug</string>
			<key>buildSettings</key>
			<dict>
				<key>PRODUCT_BUNDLE_IDENTIFIER</key>
				<string>com.example.myapp.debug</string>
				<key>CODE_SIGN_ENTITLEMENTS</key>
				<string>MyApp.debug.entitlements</string>
				<key>INFOPLIST_FILE</key>
				<string>Info.debug.plist</string>
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

const mockDebugEntitlements = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>com.apple.developer.associated-domains</key>
	<array>
		<string>applinks:debug.example.com</string>
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

const mockDebugInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.myapp.debug</string>
	<key>CFBundleURLTypes</key>
	<array>
		<dict>
			<key>CFBundleURLName</key>
			<string>myapp-debug-scheme</string>
			<key>CFBundleURLSchemes</key>
			<array>
				<string>myappdebug</string>
			</array>
		</dict>
	</array>
</dict>
</plist>`

func createMockProject(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "linkctl-test-*")
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

	err = os.WriteFile(filepath.Join(tmpDir, "MyApp.debug.entitlements"), []byte(mockDebugEntitlements), 0o644)
	if err != nil {
		t.Fatalf("failed to write MyApp.debug.entitlements: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "Info.plist"), []byte(mockInfoPlist), 0o644)
	if err != nil {
		t.Fatalf("failed to write Info.plist: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "Info.debug.plist"), []byte(mockDebugInfoPlist), 0o644)
	if err != nil {
		t.Fatalf("failed to write Info.debug.plist: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestParseXcodeProject_ReleaseConfiguration(t *testing.T) {
	projDir, cleanup := createMockProject(t)
	defer cleanup()

	// Parse project with default/Release configuration
	result, err := pbxproj.ParseXcodeProject(projDir, "Release")
	if err != nil {
		t.Fatalf("ParseXcodeProject failed: %v", err)
	}

	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(result.Targets))
	}

	target := result.Targets[0]
	if target.Name != "MyApp" {
		t.Errorf("expected target name 'MyApp', got '%s'", target.Name)
	}

	if target.BundleID != "com.example.myapp" {
		t.Errorf("expected bundle ID 'com.example.myapp', got '%s'", target.BundleID)
	}

	if target.EntitlementsPath != "MyApp.entitlements" {
		t.Errorf("expected entitlements path 'MyApp.entitlements', got '%s'", target.EntitlementsPath)
	}

	if target.InfoPlistPath != "Info.plist" {
		t.Errorf("expected info plist path 'Info.plist', got '%s'", target.InfoPlistPath)
	}

	if len(target.AssociatedDomains) != 1 || target.AssociatedDomains[0] != "applinks:example.com" {
		t.Errorf("unexpected associated domains: %v", target.AssociatedDomains)
	}

	if len(target.URLSchemes) != 1 || target.URLSchemes[0].Schemes[0] != "myapp" {
		t.Errorf("unexpected URL schemes: %v", target.URLSchemes)
	}
}

func TestParseXcodeProject_DebugConfiguration(t *testing.T) {
	projDir, cleanup := createMockProject(t)
	defer cleanup()

	// Parse project with Debug configuration explicitly
	result, err := pbxproj.ParseXcodeProject(projDir, "Debug")
	if err != nil {
		t.Fatalf("ParseXcodeProject failed: %v", err)
	}

	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(result.Targets))
	}

	target := result.Targets[0]
	if target.BundleID != "com.example.myapp.debug" {
		t.Errorf("expected bundle ID 'com.example.myapp.debug', got '%s'", target.BundleID)
	}

	if target.EntitlementsPath != "MyApp.debug.entitlements" {
		t.Errorf("expected entitlements path 'MyApp.debug.entitlements', got '%s'", target.EntitlementsPath)
	}

	if target.InfoPlistPath != "Info.debug.plist" {
		t.Errorf("expected info plist path 'Info.debug.plist', got '%s'", target.InfoPlistPath)
	}

	if len(target.AssociatedDomains) != 1 || target.AssociatedDomains[0] != "applinks:debug.example.com" {
		t.Errorf("unexpected associated domains: %v", target.AssociatedDomains)
	}

	if len(target.URLSchemes) != 1 || target.URLSchemes[0].Schemes[0] != "myappdebug" {
		t.Errorf("unexpected URL schemes: %v", target.URLSchemes)
	}
}

func TestParseXcodeProject_DefaultFallback(t *testing.T) {
	projDir, cleanup := createMockProject(t)
	defer cleanup()

	// Parse project with empty configuration should default to Release
	result, err := pbxproj.ParseXcodeProject(projDir, "")
	if err != nil {
		t.Fatalf("ParseXcodeProject failed: %v", err)
	}

	target := result.Targets[0]
	if target.BundleID != "com.example.myapp" {
		t.Errorf("expected default config to parse Release (com.example.myapp), got bundle ID '%s'", target.BundleID)
	}
}
