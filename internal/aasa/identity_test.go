package aasa_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/space-code/linkctl/internal/aasa"
)

// minimalPbxproj describes two native targets with no entitlements/Info.plist
// references — enough for IdentityFromProject, which only needs BundleID and
// TeamID off build settings.
func minimalPbxproj(targets ...struct{ id, name, bundleID, teamID string }) string {
	var targetIDs, targetDicts string
	for _, t := range targets {
		targetIDs += "<string>" + t.id + "</string>\n"
		targetDicts += `
		<key>` + t.id + `</key>
		<dict>
			<key>isa</key>
			<string>PBXNativeTarget</string>
			<key>name</key>
			<string>` + t.name + `</string>
			<key>productType</key>
			<string>com.apple.product-type.application</string>
			<key>buildConfigurationList</key>
			<string>CFGLIST_` + t.id + `</string>
			<key>dependencies</key>
			<array/>
			<key>buildPhases</key>
			<array/>
		</dict>
		<key>CFGLIST_` + t.id + `</key>
		<dict>
			<key>isa</key>
			<string>XCConfigurationList</string>
			<key>buildConfigurations</key>
			<array><string>CFG_` + t.id + `</string></array>
			<key>defaultConfigurationName</key>
			<string>Release</string>
		</dict>
		<key>CFG_` + t.id + `</key>
		<dict>
			<key>isa</key>
			<string>XCBuildConfiguration</string>
			<key>name</key>
			<string>Release</string>
			<key>buildSettings</key>
			<dict>
				<key>PRODUCT_BUNDLE_IDENTIFIER</key>
				<string>` + t.bundleID + `</string>
				<key>DEVELOPMENT_TEAM</key>
				<string>` + t.teamID + `</string>
			</dict>
		</dict>`
	}

	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>archiveVersion</key><string>1</string>
	<key>classes</key><dict/>
	<key>objectVersion</key><string>46</string>
	<key>objects</key>
	<dict>
		<key>PROJECT_ID</key>
		<dict>
			<key>isa</key><string>PBXProject</string>
			<key>buildConfigurationList</key><string>PROJECT_CFGLIST</string>
			<key>targets</key><array>` + targetIDs + `</array>
			<key>attributes</key><dict/>
		</dict>
		<key>PROJECT_CFGLIST</key>
		<dict>
			<key>isa</key><string>XCConfigurationList</string>
			<key>buildConfigurations</key><array><string>PROJECT_CFG</string></array>
			<key>defaultConfigurationName</key><string>Release</string>
		</dict>
		<key>PROJECT_CFG</key>
		<dict>
			<key>isa</key><string>XCBuildConfiguration</string>
			<key>name</key><string>Release</string>
			<key>buildSettings</key><dict/>
		</dict>` + targetDicts + `
	</dict>
	<key>rootObject</key><string>PROJECT_ID</string>
</dict>
</plist>`
}

func writeMockProject(t *testing.T, plist string) string {
	t.Helper()
	dir := t.TempDir()
	xcproj := filepath.Join(dir, "Mock.xcodeproj")
	if err := os.MkdirAll(xcproj, 0o755); err != nil {
		t.Fatalf("failed to create .xcodeproj dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(xcproj, "project.pbxproj"), []byte(plist), 0o644); err != nil {
		t.Fatalf("failed to write project.pbxproj: %v", err)
	}
	return xcproj
}

func TestIdentityFromProject_SingleTarget(t *testing.T) {
	plist := minimalPbxproj(struct{ id, name, bundleID, teamID string }{"T1", "MyApp", "com.example.app", "ABCDE12345"})
	dir := writeMockProject(t, plist)

	identity, err := aasa.IdentityFromProject(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.BundleID != "com.example.app" || identity.TeamID != "ABCDE12345" {
		t.Errorf("unexpected identity: %+v", identity)
	}
	if identity.AppID() != "ABCDE12345.com.example.app" {
		t.Errorf("unexpected AppID(): %q", identity.AppID())
	}
}

func TestIdentityFromProject_MultipleTargetsRequireFilter(t *testing.T) {
	plist := minimalPbxproj(
		struct{ id, name, bundleID, teamID string }{"T1", "MyApp", "com.example.app", "ABCDE12345"},
		struct{ id, name, bundleID, teamID string }{"T2", "MyAppExtension", "com.example.app.ext", "ABCDE12345"},
	)
	dir := writeMockProject(t, plist)

	_, err := aasa.IdentityFromProject(dir, "")
	if err == nil {
		t.Fatal("expected an error when multiple targets exist and no --target is given")
	}
}

func TestIdentityFromProject_TargetFilterSelectsCorrectTarget(t *testing.T) {
	plist := minimalPbxproj(
		struct{ id, name, bundleID, teamID string }{"T1", "MyApp", "com.example.app", "ABCDE12345"},
		struct{ id, name, bundleID, teamID string }{"T2", "MyAppExtension", "com.example.app.ext", "ABCDE12345"},
	)
	dir := writeMockProject(t, plist)

	identity, err := aasa.IdentityFromProject(dir, "MyAppExtension")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.BundleID != "com.example.app.ext" {
		t.Errorf("expected the filtered target's bundle ID, got %q", identity.BundleID)
	}
}

func TestIdentityFromProject_TargetFilterNotFound(t *testing.T) {
	plist := minimalPbxproj(struct{ id, name, bundleID, teamID string }{"T1", "MyApp", "com.example.app", "ABCDE12345"})
	dir := writeMockProject(t, plist)

	_, err := aasa.IdentityFromProject(dir, "DoesNotExist")
	if err == nil {
		t.Fatal("expected an error for a nonexistent --target")
	}
}

func TestIdentityFromProject_NoXcodeproj(t *testing.T) {
	dir := t.TempDir()
	_, err := aasa.IdentityFromProject(dir, "")
	if err == nil {
		t.Fatal("expected an error when the path contains no .xcodeproj")
	}
}
