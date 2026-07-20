package pbxproj

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-xcode/plistutil"
	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"
	"github.com/bitrise-io/go-xcode/xcodeproject/xcodeproj"
	"github.com/space-code/linkctl/internal/models"
)

func ParseXcodeProject(projectPath string, configuration string) (*models.XcodeProject, error) {
	xcprojPath, err := resolveXcodeprojDir(projectPath)
	if err != nil {
		return nil, err
	}
	projectRoot := filepath.Dir(xcprojPath)

	proj, err := xcodeproj.Open(xcprojPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", xcprojPath, err)
	}

	result := &models.XcodeProject{
		Path: filepath.Join(xcprojPath, "project.pbxproj"),
	}

	for _, target := range proj.Proj.Targets {
		if target.Type != xcodeproj.NativeTargetType {
			continue
		}

		if isTestTarget(target.ProductType) {
			continue
		}

		t := models.XcodeTarget{Name: target.Name}

		applyBuildSettings(&t, proj, target, configuration)

		enrichTarget(&t, projectRoot)

		result.Targets = append(result.Targets, t)
	}

	return result, nil
}

func isTestTarget(productType string) bool {
	switch productType {
	case "com.apple.product-type.bundle.unit-test",
		"com.apple.product-type.bundle.ui-testing":
		return true
	}
	return false
}

func applyBuildSettings(
	t *models.XcodeTarget,
	proj xcodeproj.XcodeProj,
	target xcodeproj.Target,
	configuration string,
) {
	// 1. Determine configuration to use
	config := configuration
	if config == "" {
		config = target.BuildConfigurationList.DefaultConfigurationName
	}
	if config == "" {
		config = proj.Proj.BuildConfigurationList.DefaultConfigurationName
	}
	if config == "" {
		if len(target.BuildConfigurationList.BuildConfigurations) > 0 {
			config = target.BuildConfigurationList.BuildConfigurations[0].Name
		} else {
			config = "Release"
		}
	}

	// 2. Query bundle ID
	if bundleID, err := proj.TargetBundleID(target.Name, config); err == nil && bundleID != "" {
		t.BundleID = bundleID
	}

	// 3. Query entitlements path
	if entPath, err := proj.TargetCodeSignEntitlementsPath(target.Name, config); err == nil && entPath != "" {
		t.EntitlementsPath = entPath
	}

	// 4. Query info plist path
	if infoPath, err := proj.TargetInfoplistPath(target.Name, config); err == nil && infoPath != "" {
		t.InfoPlistPath = infoPath
	}

	// 5. Query generate info plist file setting
	if bs, err := proj.TargetBuildSettings(target.Name, config); err == nil {
		if raw, err := bs.String("GENERATE_INFOPLIST_FILE"); err == nil {
			t.GeneratesInfoPlist = strings.EqualFold(raw, "yes")
		}
	}

	// Fallback to manual first configuration parsing if any value is still empty
	if t.BundleID == "" || t.EntitlementsPath == "" || t.InfoPlistPath == "" {
		for _, cfg := range target.BuildConfigurationList.BuildConfigurations {
			fillFromBuildSettings(t, cfg.BuildSettings)
			break
		}
		if t.EntitlementsPath == "" {
			for _, cfg := range proj.Proj.BuildConfigurationList.BuildConfigurations {
				if raw, err := cfg.BuildSettings.String("CODE_SIGN_ENTITLEMENTS"); err == nil && raw != "" {
					t.EntitlementsPath = raw
				}
			}
		}
	}
}

func fillFromBuildSettings(t *models.XcodeTarget, bs serialized.Object) {
	if t.BundleID == "" {
		if raw, err := bs.String("PRODUCT_BUNDLE_IDENTIFIER"); err == nil && raw != "" {
			t.BundleID = resolveVar(raw, bs)
		}
	}
	if t.EntitlementsPath == "" {
		if raw, err := bs.String("CODE_SIGN_ENTITLEMENTS"); err == nil && raw != "" {
			t.EntitlementsPath = raw
		}
	}
	if t.InfoPlistPath == "" {
		if raw, err := bs.String("INFOPLIST_FILE"); err == nil && raw != "" {
			t.InfoPlistPath = normalizePath(raw, t.Name)
		}
	}
	if !t.GeneratesInfoPlist {
		if raw, err := bs.String("GENERATE_INFOPLIST_FILE"); err == nil {
			t.GeneratesInfoPlist = strings.EqualFold(raw, "yes")
		}
	}
}

func resolveVar(raw string, bs serialized.Object) string {
	re := regexp.MustCompile(`[$][{(]([^$:})]+)(?::[^$)}]*)??[)}]`)
	seen := map[string]bool{}
	cur := raw
	for strings.Contains(cur, "$") {
		m := re.FindStringSubmatchIndex(cur)
		if m == nil {
			break
		}
		varName := cur[m[2]:m[3]]
		val, err := bs.String(varName)
		if err != nil {
			break
		}
		next := cur[:m[0]] + val + cur[m[1]:]
		if seen[next] {
			break
		}
		seen[next] = true
		cur = next
	}
	return cur
}

func enrichTarget(t *models.XcodeTarget, projectRoot string) {
	if t.EntitlementsPath != "" {
		var absEnt string
		if filepath.IsAbs(t.EntitlementsPath) {
			absEnt = t.EntitlementsPath
		} else {
			raw := strings.ReplaceAll(t.EntitlementsPath, "$(TARGET_NAME)", t.Name)
			raw = strings.ReplaceAll(raw, "${TARGET_NAME}", t.Name)
			absEnt = entitlementsPath(raw, projectRoot)
		}
		if domains, err := parseEntitlements(absEnt); err == nil {
			t.AssociatedDomains = domains
		}
		if rel, err := filepath.Rel(projectRoot, absEnt); err == nil {
			t.EntitlementsPath = rel
		}
	}
	if t.InfoPlistPath != "" {
		var absInfo string
		if filepath.IsAbs(t.InfoPlistPath) {
			absInfo = t.InfoPlistPath
		} else {
			absInfo = filepath.Join(projectRoot, t.InfoPlistPath)
		}
		if schemes, bundleID, err := parseInfoPlist(absInfo); err == nil {
			t.URLSchemes = schemes
			if t.BundleID == "" && bundleID != "" {
				t.BundleID = bundleID
			}
		}
		if rel, err := filepath.Rel(projectRoot, absInfo); err == nil {
			t.InfoPlistPath = rel
		}
	}
}

func resolveXcodeprojDir(startPath string) (string, error) {
	if strings.HasSuffix(startPath, ".xcodeproj") {
		if _, err := os.Stat(filepath.Join(startPath, "project.pbxproj")); err == nil {
			return startPath, nil
		}
	}

	entries, err := os.ReadDir(startPath)
	if err != nil {
		return "", fmt.Errorf("cannot read %q: %w", startPath, err)
	}

	// Direct child .xcodeproj.
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".xcodeproj") {
			c := filepath.Join(startPath, e.Name())
			if _, err := os.Stat(filepath.Join(c, "project.pbxproj")); err == nil {
				return c, nil
			}
		}
	}

	// One level deeper (monorepo layout: repo/ios/MyApp.xcodeproj).
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, _ := os.ReadDir(filepath.Join(startPath, e.Name()))
		for _, se := range sub {
			if strings.HasSuffix(se.Name(), ".xcodeproj") {
				c := filepath.Join(startPath, e.Name(), se.Name())
				if _, err := os.Stat(filepath.Join(c, "project.pbxproj")); err == nil {
					return c, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no .xcodeproj found in %q", startPath)
}

// normalizePath strips common build-variable prefixes from a pbxproj path value.
// Used for INFOPLIST_FILE and similar settings that do not need abs resolution.
func normalizePath(p, targetName string) string {
	p = strings.ReplaceAll(p, "$(SRCROOT)/", "")
	p = strings.ReplaceAll(p, "$(SRCROOT)", "")
	p = strings.ReplaceAll(p, "$(TARGET_NAME)", targetName)
	return p
}

// parseEntitlements reads a .entitlements plist and returns the values of
// com.apple.developer.associated-domains.
//
// Returns (nil, nil) when the key is absent — not every target uses Universal
// Links, and absence is normal for custom-scheme-only apps.
func parseEntitlements(path string) ([]string, error) {
	data, err := plistutil.NewPlistDataFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read entitlements %q: %w", path, err)
	}

	domains, ok := data.GetStringArray("com.apple.developer.associated-domains")
	if !ok {
		return nil, nil // key absent — not an error
	}
	return domains, nil
}

// parseInfoPlist reads an Info.plist and returns URL schemes + bundle ID.
//
// bundleID is "" when the plist contains a build-variable placeholder such as
// $(PRODUCT_BUNDLE_IDENTIFIER) — the authoritative value comes from pbxproj
// build settings in that case.
func parseInfoPlist(path string) ([]models.URLScheme, string, error) {
	data, err := plistutil.NewPlistDataFromFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("cannot read Info.plist %q: %w", path, err)
	}

	bundleID, _ := data.GetString("CFBundleIdentifier")
	if strings.HasPrefix(bundleID, "$(") || strings.HasPrefix(bundleID, "${") {
		bundleID = ""
	}

	schemes := parseURLTypes(data)
	return schemes, bundleID, nil
}

// parseURLTypes extracts CFBundleURLTypes entries from parsed plist data.
func parseURLTypes(data plistutil.PlistData) []models.URLScheme {
	entries, ok := data.GetMapStringInterfaceArray("CFBundleURLTypes")
	if !ok {
		return nil
	}

	var schemes []models.URLScheme
	for _, entry := range entries {
		name, _ := entry.GetString("CFBundleURLName")
		rawSchemes, ok := entry.GetStringArray("CFBundleURLSchemes")
		if !ok || len(rawSchemes) == 0 {
			continue
		}
		schemes = append(schemes, models.URLScheme{Name: name, Schemes: rawSchemes})
	}
	return schemes
}

// entitlementsPath resolves a raw CODE_SIGN_ENTITLEMENTS build setting value
// to an absolute path on disk.
//
// projectRoot is the directory that contains the .xcodeproj bundle
// (i.e. filepath.Dir(xcprojPath)), which is identical to Xcode's $(SRCROOT).
//
// Patterns seen in real projects, all handled here:
//
//	MyApp/MyApp.entitlements                       — plain relative path
//	$(SRCROOT)/MyApp/MyApp.entitlements            — explicit SRCROOT with slash
//	$(SRCROOT)MyApp/MyApp.entitlements             — explicit SRCROOT without slash
//	$(TARGET_NAME)/$(TARGET_NAME).entitlements     — target-name variables
//	MyApp/$(TARGET_NAME).entitlements              — partial variable
//	/Users/runner/work/MyApp/MyApp.entitlements    — absolute (xcodebuild output)
func entitlementsPath(raw, projectRoot string) string {
	// Absolute paths need no further processing (come from xcodebuild layer).
	if filepath.IsAbs(raw) {
		return raw
	}

	// Expand the two variables that appear in this setting.
	// $(SRCROOT) == projectRoot; $(TARGET_NAME) is unknowable here without the
	// target name, so we strip it and let the caller handle the mismatch via
	// the filesystem fallback in enrichTarget.
	raw = strings.ReplaceAll(raw, "$(SRCROOT)/", "")
	raw = strings.ReplaceAll(raw, "$(SRCROOT)", "")
	raw = strings.ReplaceAll(raw, "${SRCROOT}/", "")
	raw = strings.ReplaceAll(raw, "${SRCROOT}", "")

	// After stripping SRCROOT the path may start with a slash if the original
	// value was "$(SRCROOT)/…" and the variable included the slash in the value.
	// filepath.Join handles this correctly.
	return filepath.Join(projectRoot, raw)
}
