// Package models contains all domain types shared across packages.
// No logic lives here — only data structures and constants.
package models

// DeepLink is a parsed representation of a raw URL or custom-scheme URI.
type DeepLink struct {
	Raw      string            `json:"raw"`
	Scheme   string            `json:"scheme"`
	Host     string            `json:"host"`
	Path     string            `json:"path"`
	Query    map[string]string `json:"query,omitempty"`
	Fragment string            `json:"fragment,omitempty"`
	Type     LinkType          `json:"type"`
}

// LinkType classifies the deep link by platform handling mechanism.
type LinkType string

const (
	// LinkTypeUniversalLink is an https link handled by iOS via AASA.
	LinkTypeUniversalLink LinkType = "Universal Link"
	// LinkTypeAppLink is an https link handled by Android via assetlinks.json.
	LinkTypeAppLink LinkType = "App Link"
	// LinkTypeCustomScheme is a non-http URI handled by any app that registers the scheme.
	LinkTypeCustomScheme LinkType = "Custom Scheme"
)

// ValidationResult is a single check outcome produced by the validator or checker.
type ValidationResult struct {
	Check   string `json:"check"`
	Status  Status `json:"status"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Status is the outcome of a single validation check.
type Status string

const (
	StatusPass    Status = "PASS"
	StatusFail    Status = "FAIL"
	StatusWarning Status = "WARN"
	StatusInfo    Status = "INFO"
	StatusSkip    Status = "SKIP"
)

// SimulationResult is the outcome of opening a deep link on a device or emulator.
type SimulationResult struct {
	Platform   string `json:"platform"` // "iOS" or "Android"
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name,omitempty"`
	Success    bool   `json:"success"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// CacheResetResult is the outcome of a full cache-reset sequence.
// Steps are executed sequentially; the first fatal failure stops the chain.
type CacheResetResult struct {
	Platform string           `json:"platform"` // "iOS" or "Android"
	DeviceID string           `json:"device_id"`
	Steps    []CacheResetStep `json:"steps"`
	Success  bool             `json:"success"`
	Error    string           `json:"error,omitempty"`
}

// CacheResetStep is the result of one command within a cache-reset sequence.
type CacheResetStep struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AASAFile is the JSON shape of apple-app-site-association, served at
// https://<host>/.well-known/apple-app-site-association
type AASAFile struct {
	AppLinks struct {
		// Apps must be an empty array per Apple spec — non-empty breaks Universal Links.
		Apps    []string     `json:"apps"`
		Details []AASADetail `json:"details"`
	} `json:"applinks"`
	WebCredentials *struct {
		Apps []string `json:"apps"`
	} `json:"webcredentials,omitempty"`
}

// AASADetail is one entry in the applinks.details array.
type AASADetail struct {
	AppID      string          `json:"appID"`                // "<TeamID>.<BundleID>"
	Paths      []string        `json:"paths,omitempty"`      // legacy format
	Components []AASAComponent `json:"components,omitempty"` // modern format (iOS 13+)
}

// AASAComponent is a modern path-matcher in AASA (iOS 13+).
type AASAComponent struct {
	Path     string `json:"/,omitempty"`
	Query    string `json:"?,omitempty"`
	Fragment string `json:"#,omitempty"`
	Comment  string `json:"comment,omitempty"`
	Exclude  bool   `json:"exclude,omitempty"`
}

// AssetLinksFile is the JSON shape of /.well-known/assetlinks.json.
type AssetLinksFile []AssetLink

// AssetLink is one entry in assetlinks.json.
type AssetLink struct {
	Relation []string        `json:"relation"`
	Target   AssetLinkTarget `json:"target"`
}

// AssetLinkTarget identifies an Android app in assetlinks.json.
type AssetLinkTarget struct {
	Namespace              string   `json:"namespace"`
	PackageName            string   `json:"package_name"`
	SHA256CertFingerprints []string `json:"sha256_cert_fingerprints"`
}

// XcodeProject holds data extracted from a .xcodeproj bundle.
type XcodeProject struct {
	Path    string        `json:"path"`
	Targets []XcodeTarget `json:"targets"`
}

// XcodeTarget is one PBXNativeTarget within an Xcode project.
type XcodeTarget struct {
	Name     string `json:"name"`
	BundleID string `json:"bundle_id,omitempty"`

	// Paths are project-relative (e.g. "MyApp/MyApp.entitlements").
	EntitlementsPath string `json:"entitlements_path,omitempty"`
	InfoPlistPath    string `json:"info_plist_path,omitempty"`

	// AssociatedDomains are raw entitlement values: "applinks:example.com",
	// "webcredentials:example.com", "applinks:*.example.com?mode=developer", …
	AssociatedDomains []string    `json:"associated_domains,omitempty"`
	URLSchemes        []URLScheme `json:"url_schemes,omitempty"`

	// GeneratesInfoPlist is true when GENERATE_INFOPLIST_FILE = YES in the
	// pbxproj. Modern Xcode (13+) projects build the Info.plist entirely from
	// build settings at compile time — no static Info.plist exists on disk.
	GeneratesInfoPlist bool `json:"generates_info_plist,omitempty"`
}

// URLScheme represents one CFBundleURLTypes entry from Info.plist.
type URLScheme struct {
	Name    string   `json:"name,omitempty"` // CFBundleURLName
	Schemes []string `json:"schemes"`        // CFBundleURLSchemes
}

// AndroidManifest holds data extracted from AndroidManifest.xml.
type AndroidManifest struct {
	Path        string            `json:"path"`
	PackageName string            `json:"package_name"`
	Activities  []AndroidActivity `json:"activities,omitempty"`
}

// AndroidActivity represents one <activity> element.
type AndroidActivity struct {
	Name          string         `json:"name"`
	IntentFilters []IntentFilter `json:"intent_filters,omitempty"`
}

// IntentFilter represents one <intent-filter> element.
type IntentFilter struct {
	AutoVerify  bool     `json:"auto_verify,omitempty"`
	Actions     []string `json:"actions,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	DataSchemes []string `json:"data_schemes,omitempty"`
	DataHosts   []string `json:"data_hosts,omitempty"`
	DataPaths   []string `json:"data_paths,omitempty"`
}

// AppCheckReport is the output of `deeplink check-app`.
type AppCheckReport struct {
	ProjectPath string `json:"project_path"`
	Platform    string `json:"platform"` // "ios" or "android"

	// One of these is populated depending on platform.
	XcodeProject    *XcodeProject    `json:"xcode_project,omitempty"`
	AndroidManifest *AndroidManifest `json:"android_manifest,omitempty"`

	Link    *DeepLink          `json:"link"`
	Checks  []ValidationResult `json:"checks"`
	Summary AppCheckSummary    `json:"summary"`
}

// AppCheckSummary aggregates check counts from AppCheckReport.
type AppCheckSummary struct {
	Total    int  `json:"total"`
	Passed   int  `json:"passed"`
	Failed   int  `json:"failed"`
	Warnings int  `json:"warnings"`
	OK       bool `json:"ok"` // true when Failed == 0
}

// ProjectScan is the output of `deeplink scan`.
// Unlike AppCheckReport it is not tied to a specific link.
type ProjectScan struct {
	ProjectPath     string           `json:"project_path"`
	Platform        string           `json:"platform"` // "ios" or "android"
	Targets         []TargetSummary  `json:"targets,omitempty"`
	RegisteredLinks []RegisteredLink `json:"registered_links,omitempty"`
}

// TargetSummary is the per-target view shown in scan output.
type TargetSummary struct {
	Name              string   `json:"name"`
	AssociatedDomains []string `json:"associated_domains,omitempty"`
	URLSchemes        []string `json:"url_schemes,omitempty"` // flat, e.g. ["myapp", "myapp-dev"]
	EntitlementsPath  string   `json:"entitlements_path,omitempty"`
	InfoPlistPath     string   `json:"info_plist_path,omitempty"`
}

// RegisteredLink is one deep link pattern that an app target can handle.
type RegisteredLink struct {
	Pattern  string `json:"pattern"`   // e.g. "https://example.com/**" or "myapp://**"
	Target   string `json:"target"`    // Xcode target name or Android activity class
	LinkType string `json:"link_type"` // "Universal Link", "App Link", "Custom Scheme"
}

// Used by --json output and the embedded Web UI (/api/debug endpoint).
// Separate from the internal types so the JSON shape can evolve independently.

// JSONReport is the top-level envelope for --json output.
type JSONReport struct {
	Link        JSONLink         `json:"link"`
	Validations []JSONValidation `json:"validations"`
	Simulations []JSONSimulation `json:"simulations,omitempty"`
	Summary     JSONSummary      `json:"summary"`
	GeneratedAt string           `json:"generated_at"` // RFC3339
}

// JSONLink is the wire representation of a parsed deep link.
type JSONLink struct {
	Raw      string            `json:"raw"`
	Scheme   string            `json:"scheme"`
	Host     string            `json:"host"`
	Path     string            `json:"path"`
	Query    map[string]string `json:"query,omitempty"`
	Fragment string            `json:"fragment,omitempty"`
	Type     string            `json:"type"` // string, not LinkType, for forward compatibility
}

// JSONValidation is the wire representation of one validation result.
type JSONValidation struct {
	Check   string `json:"check"`
	Status  string `json:"status"` // "PASS", "FAIL", "WARN", "INFO", "SKIP"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// JSONSimulation is the wire representation of one simulation result.
type JSONSimulation struct {
	Platform string `json:"platform"`
	DeviceID string `json:"device_id"`
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

// JSONSummary aggregates counts for the JSON report header.
type JSONSummary struct {
	Total    int  `json:"total"`
	Passed   int  `json:"passed"`
	Failed   int  `json:"failed"`
	Warnings int  `json:"warnings"`
	OK       bool `json:"ok"`
}
