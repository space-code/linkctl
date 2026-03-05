package appcheck

import (
	"fmt"
	"os"
	"strings"

	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/pbxproj"
)

func CheckApp(projectPath string, link *models.DeepLink, targetFilter string, configuration string) (*models.AppCheckReport, error) {
	report := &models.AppCheckReport{
		ProjectPath: projectPath,
		Link:        link,
	}

	platform, err := detectPlatform(projectPath)
	if err != nil {
		return nil, err
	}
	report.Platform = platform

	switch platform {
	case "ios":
		proj, err := pbxproj.ParseXcodeProject(projectPath, configuration)
		if err != nil {
			return nil, err
		}
		report.XcodeProject = proj
		report.Checks = checkIOS(proj, link, targetFilter)
	}

	for _, c := range report.Checks {
		report.Summary.Total++
		switch c.Status {
		case models.StatusPass:
			report.Summary.Passed++
		case models.StatusFail:
			report.Summary.Failed++
		case models.StatusWarning:
			report.Summary.Warnings++
		}
	}
	report.Summary.OK = report.Summary.Failed == 0

	return report, nil
}

// ScanProject returns all deep link patterns registered in the project,
// without validating a specific link.
func ScanProject(projectPath string) (*models.ProjectScan, error) {
	platform, err := detectPlatform(projectPath)
	if err != nil {
		return nil, err
	}

	scan := &models.ProjectScan{
		ProjectPath: projectPath,
		Platform:    platform,
	}

	switch platform {
	case "ios":
		proj, err := pbxproj.ParseXcodeProject(projectPath, "")
		if err != nil {
			return nil, err
		}
		for _, t := range proj.Targets {
			ts := models.TargetSummary{
				Name:              t.Name,
				AssociatedDomains: t.AssociatedDomains,
				EntitlementsPath:  t.EntitlementsPath,
				InfoPlistPath:     t.InfoPlistPath,
			}
			for _, us := range t.URLSchemes {
				ts.URLSchemes = append(ts.URLSchemes, us.Schemes...)
			}
			scan.Targets = append(scan.Targets, ts)

			for _, domain := range t.AssociatedDomains {
				if strings.HasPrefix(domain, "applinks:") {
					host := strings.TrimPrefix(domain, "applinks:")
					host = strings.SplitN(host, "?", 2)[0]
					scan.RegisteredLinks = append(scan.RegisteredLinks, models.RegisteredLink{
						Pattern:  fmt.Sprintf("https://%s/**", host),
						Target:   t.Name,
						LinkType: "Universal Link",
					})
				} else if strings.HasPrefix(domain, "https://") || strings.HasPrefix(domain, "http://") {
					// Malformed entry — full URL instead of "applinks:host".
					// Include it so the user sees it, marked as misconfigured.
					scan.RegisteredLinks = append(scan.RegisteredLinks, models.RegisteredLink{
						Pattern:  domain,
						Target:   t.Name,
						LinkType: "⚠ Bad Format (missing applinks: prefix)",
					})
				}
			}
			for _, us := range t.URLSchemes {
				for _, s := range us.Schemes {
					scan.RegisteredLinks = append(scan.RegisteredLinks, models.RegisteredLink{
						Pattern:  fmt.Sprintf("%s://**", s),
						Target:   t.Name,
						LinkType: "Custom Scheme",
					})
				}
			}
		}
	}

	return scan, nil
}

// checkIOS is the top-level iOS validator.
//
// When targetFilter is non-empty only the matching target is validated;
// otherwise every native target is validated and a cross-target scheme-conflict
// warning is emitted when applicable.
func checkIOS(proj *models.XcodeProject, link *models.DeepLink, targetFilter string) []models.ValidationResult {
	var results []models.ValidationResult

	if len(proj.Targets) == 0 {
		return append(results, models.ValidationResult{
			Check:   "Targets Found",
			Status:  models.StatusFail,
			Message: "No targets found in Xcode project",
			Detail:  "Make sure you pointed to a valid .xcodeproj directory",
		})
	}

	targets, notFoundErr := selectTargets(proj.Targets, targetFilter)
	if notFoundErr != nil {
		return append(results, *notFoundErr)
	}

	noun := "target"
	if len(proj.Targets) != 1 {
		noun = "targets"
	}
	summary := fmt.Sprintf("%d %s found", len(proj.Targets), noun)
	if targetFilter != "" {
		summary += fmt.Sprintf(" — checking %q", targets[0].Name)
	}
	results = append(results, models.ValidationResult{
		Check:   "Targets Found",
		Status:  models.StatusPass,
		Message: summary,
	})

	isUniversalLink := link.Scheme == "https" || link.Scheme == "http"

	for _, target := range targets {
		prefix := fmt.Sprintf("[%s]", target.Name)
		results = append(results, checkIOSTarget(target, link, prefix, isUniversalLink)...)
	}

	// Cross-target scheme conflict — only meaningful when all targets are checked.
	if !isUniversalLink && targetFilter == "" {
		if conflicting := targetsWithScheme(proj.Targets, link.Scheme); len(conflicting) > 1 {
			results = append(results, models.ValidationResult{
				Check:   "Scheme Conflict",
				Status:  models.StatusWarning,
				Message: fmt.Sprintf("Scheme '%s' is registered in multiple targets: %s", link.Scheme, strings.Join(conflicting, ", ")),
				Detail:  "Multiple targets with the same scheme may cause unexpected routing",
			})
		}
	}

	return results
}

// selectTargets returns the targets to validate.
// filter="" → all targets.
// filter set → the single matching target, or a FAIL result listing what's available.
func selectTargets(targets []models.XcodeTarget, filter string) ([]models.XcodeTarget, *models.ValidationResult) {
	if filter == "" {
		return targets, nil
	}
	for _, t := range targets {
		if strings.EqualFold(t.Name, filter) {
			return []models.XcodeTarget{t}, nil
		}
	}
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.Name
	}
	return nil, &models.ValidationResult{
		Check:   "Target Found",
		Status:  models.StatusFail,
		Message: fmt.Sprintf("Target %q not found in project", filter),
		Detail:  "Available targets: " + strings.Join(names, ", "),
	}
}

// checkIOSTarget validates a single Xcode target against the link.
func checkIOSTarget(target models.XcodeTarget, link *models.DeepLink, prefix string, isUniversalLink bool) []models.ValidationResult {
	var results []models.ValidationResult

	if target.EntitlementsPath == "" {
		status := models.StatusInfo
		msg := "No entitlements file (not required for custom schemes)"
		detail := ""
		if isUniversalLink {
			status = models.StatusFail
			msg = "No entitlements file found"
			detail = "Universal Links require an entitlements file with associated-domains"
		}
		results = append(results, models.ValidationResult{
			Check:   prefix + " Entitlements",
			Status:  status,
			Message: msg,
			Detail:  detail,
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Entitlements",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("Found: %s", target.EntitlementsPath),
		})
	}

	if isUniversalLink {
		results = append(results, checkAssociatedDomains(target, link, prefix)...)
	} else {
		results = append(results, checkURLSchemes(target, link, prefix)...)
	}

	// Bundle ID — informational only.
	if target.BundleID != "" {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Bundle ID",
			Status:  models.StatusInfo,
			Message: target.BundleID,
		})
	}

	return results
}

// checkAssociatedDomains validates Universal Link domain registration.
func checkAssociatedDomains(target models.XcodeTarget, link *models.DeepLink, prefix string) []models.ValidationResult {
	var results []models.ValidationResult

	if len(target.AssociatedDomains) == 0 {
		return append(results, models.ValidationResult{
			Check:   prefix + " Associated Domains",
			Status:  models.StatusFail,
			Message: "No associated domains configured",
			Detail:  fmt.Sprintf("Add 'applinks:%s' to your entitlements file", link.Host),
		})
	}

	results = append(results, models.ValidationResult{
		Check:   prefix + " Associated Domains",
		Status:  models.StatusPass,
		Message: fmt.Sprintf("%d domain(s): %s", len(target.AssociatedDomains), strings.Join(target.AssociatedDomains, ", ")),
	})

	// Does an applinks: entry cover this host?
	if matched, matchedDomain := domainMatchesHost(target.AssociatedDomains, link.Host, "applinks"); matched {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Host Registered",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("'%s' covered by '%s'", link.Host, matchedDomain),
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Host Registered",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("Host '%s' not found in associated domains", link.Host),
			Detail:  fmt.Sprintf("Add 'applinks:%s' to %s", link.Host, target.EntitlementsPath),
		})
	}

	// Common mistake: bare domain without applinks: prefix.
	for _, d := range target.AssociatedDomains {
		if d == link.Host || d == "https://"+link.Host {
			results = append(results, models.ValidationResult{
				Check:   prefix + " Domain Format",
				Status:  models.StatusFail,
				Message: fmt.Sprintf("'%s' is missing the 'applinks:' prefix", d),
				Detail:  fmt.Sprintf("Change to 'applinks:%s'", link.Host),
			})
		}
	}

	// Informational: webcredentials also present for this host.
	if _, wc := domainMatchesHost(target.AssociatedDomains, link.Host, "webcredentials"); wc != "" {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Web Credentials",
			Status:  models.StatusInfo,
			Message: "webcredentials also configured (Shared Web Credentials / Passkeys)",
		})
	}

	return results
}

// checkURLSchemes validates custom scheme registration via Info.plist.
func checkURLSchemes(target models.XcodeTarget, link *models.DeepLink, prefix string) []models.ValidationResult {
	var results []models.ValidationResult

	if target.InfoPlistPath == "" && !target.GeneratesInfoPlist {
		return append(results, models.ValidationResult{
			Check:   prefix + " Info.plist",
			Status:  models.StatusFail,
			Message: "Info.plist not found — cannot verify URL scheme registration",
			Detail:  "Set INFOPLIST_FILE or GENERATE_INFOPLIST_FILE in build settings",
		})
	}

	if target.InfoPlistPath != "" {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Info.plist",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("Found: %s", target.InfoPlistPath),
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Info.plist",
			Status:  models.StatusInfo,
			Message: "Info.plist generated at build time (GENERATE_INFOPLIST_FILE = YES)",
			Detail:  "URL schemes must be set via INFOPLIST_KEY_CFBundleURLTypes or a separate Info.plist",
		})
	}

	if len(target.URLSchemes) == 0 {
		return append(results, models.ValidationResult{
			Check:   prefix + " URL Types",
			Status:  models.StatusFail,
			Message: "No CFBundleURLTypes found in Info.plist",
			Detail:  fmt.Sprintf("Add '%s' to URL Types in the target's Info settings", link.Scheme),
		})
	}

	allSchemes := collectAllSchemes(target.URLSchemes)
	results = append(results, models.ValidationResult{
		Check:   prefix + " URL Types",
		Status:  models.StatusPass,
		Message: fmt.Sprintf("%d scheme(s): %s", len(allSchemes), strings.Join(allSchemes, ", ")),
	})

	found := false
	for _, s := range allSchemes {
		if strings.EqualFold(s, link.Scheme) {
			found = true
			break
		}
	}
	if found {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Scheme Registered",
			Status:  models.StatusPass,
			Message: fmt.Sprintf("Scheme '%s://' is registered", link.Scheme),
		})
	} else {
		results = append(results, models.ValidationResult{
			Check:   prefix + " Scheme Registered",
			Status:  models.StatusFail,
			Message: fmt.Sprintf("Scheme '%s://' is NOT registered", link.Scheme),
			Detail:  fmt.Sprintf("Add '%s' to CFBundleURLSchemes in Info.plist", link.Scheme),
		})
	}

	return results
}

func detectPlatform(path string) (string, error) {
	if strings.HasSuffix(path, ".xcodeproj") {
		return "ios", nil
	}

	entries, err := readDirNames(path)
	if err != nil {
		return "", fmt.Errorf("cannot read %q: %w", path, err)
	}

	androidFiles := map[string]bool{
		"AndroidManifest.xml": true,
		"build.gradle":        true,
		"build.gradle.kts":    true,
		"settings.gradle":     true,
	}

	for _, e := range entries {
		if strings.HasSuffix(e, ".xcodeproj") {
			return "ios", nil
		}
		if androidFiles[e] {
			return "android", nil
		}
	}

	// One level deeper.
	for _, e := range entries {
		sub, err := readDirNames(path + "/" + e)
		if err != nil {
			continue
		}
		for _, se := range sub {
			if strings.HasSuffix(se, ".xcodeproj") {
				return "ios", nil
			}
			if androidFiles[se] {
				return "android", nil
			}
		}
	}

	return "", fmt.Errorf(
		"could not detect platform in %q\n"+
			"Point to a directory containing .xcodeproj (iOS) or AndroidManifest.xml / build.gradle (Android)",
		path,
	)
}

func readDirNames(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// domainMatchesHost reports whether host is covered by any entry in domains
// with the given service prefix ("applinks" or "webcredentials").
// Handles exact match and wildcard (*.) entries.
// Strips ?mode=developer suffixes added by Xcode Managed Capabilities.
func domainMatchesHost(domains []string, host, prefix string) (bool, string) {
	for _, d := range domains {
		pfx := prefix + ":"
		if !strings.HasPrefix(d, pfx) {
			continue
		}
		pattern := strings.TrimPrefix(d, pfx)
		pattern = strings.SplitN(pattern, "?", 2)[0] // strip ?mode=developer

		if pattern == host {
			return true, d
		}
		if strings.HasPrefix(pattern, "*.") {
			base := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(host, "."+base) {
				return true, d
			}
		}
	}
	return false, ""
}

func collectAllSchemes(urlSchemes []models.URLScheme) []string {
	var all []string
	for _, us := range urlSchemes {
		all = append(all, us.Schemes...)
	}
	return all
}

func targetsWithScheme(targets []models.XcodeTarget, scheme string) []string {
	var names []string
	for _, t := range targets {
		for _, us := range t.URLSchemes {
			for _, s := range us.Schemes {
				if strings.EqualFold(s, scheme) {
					names = append(names, t.Name)
				}
			}
		}
	}
	return names
}
