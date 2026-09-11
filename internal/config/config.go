// Package config loads linkctl.yml, the file that lets `linkctl ci` check
// a whole list of links in one CI step instead of one `linkctl` invocation
// per link.
package config

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Expect values for a LinkConfig entry.
const (
	ExpectAuto          = "" // infer from the URL's scheme
	ExpectUniversalLink = "universal-link"
	ExpectOneLink       = "onelink"
	ExpectCustomScheme  = "custom-scheme"
)

// Config is the top-level shape of linkctl.yml.
type Config struct {
	Project    string       `yaml:"project,omitempty"`
	Target     string       `yaml:"target,omitempty"`
	BundleID   string       `yaml:"bundleId,omitempty"`
	TeamID     string       `yaml:"teamId,omitempty"`
	UserAgent  string       `yaml:"userAgent,omitempty"`
	TimeoutRaw string       `yaml:"timeout,omitempty"`
	Links      []LinkConfig `yaml:"links"`
	Checks     ChecksConfig `yaml:"checks,omitempty"`
}

// LinkConfig is one entry in the top-level "links" list.
type LinkConfig struct {
	URL string `yaml:"url"`

	// Expect selects which checks run against URL. Empty means "infer from
	// scheme": https/http -> universal-link, anything else -> custom-scheme.
	// OneLink links must set this explicitly to "onelink" — a URL's host
	// alone isn't a reliable signal once branded domains are involved.
	Expect string `yaml:"expect,omitempty"`

	// FinalHost, when set (onelink entries only), is checked against the
	// host the redirect chain actually lands on.
	FinalHost string `yaml:"finalHost,omitempty"`
}

// ChecksConfig toggles optional check categories. A nil pointer means
// "use the default" (true for all three) rather than "false", so a config
// that omits "checks" entirely still runs the full suite.
type ChecksConfig struct {
	AASA       *bool `yaml:"aasa,omitempty"`
	AppleCDN   *bool `yaml:"appleCdn,omitempty"`
	AppProject *bool `yaml:"appProject,omitempty"`
}

func boolOrDefault(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// AASAEnabled reports whether AASA checks should run (default true).
func (c ChecksConfig) AASAEnabled() bool { return boolOrDefault(c.AASA, true) }

// AppleCDNEnabled reports whether the Apple CDN cross-check should run
// (default true).
func (c ChecksConfig) AppleCDNEnabled() bool { return boolOrDefault(c.AppleCDN, true) }

// AppProjectEnabled reports whether the Xcode-project-side check should run
// against each link (default true; has no effect when Project is unset).
func (c ChecksConfig) AppProjectEnabled() bool { return boolOrDefault(c.AppProject, true) }

// Timeout parses TimeoutRaw, defaulting to def when unset.
func (c Config) Timeout(def time.Duration) (time.Duration, error) {
	if c.TimeoutRaw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(c.TimeoutRaw)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", c.TimeoutRaw, err)
	}
	return d, nil
}

// Load reads and parses path, rejecting unknown fields so a typo'd key
// fails loudly instead of being silently ignored.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is an explicit --config CLI argument, not attacker input
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks the loaded config for obvious mistakes that would
// otherwise surface as a confusing failure deep inside `ci`.
func (c Config) Validate() error {
	if len(c.Links) == 0 {
		return fmt.Errorf("no links configured")
	}
	for i, l := range c.Links {
		if l.URL == "" {
			return fmt.Errorf("links[%d]: url is required", i)
		}
		switch l.Expect {
		case ExpectAuto, ExpectUniversalLink, ExpectOneLink, ExpectCustomScheme:
		default:
			return fmt.Errorf("links[%d]: invalid expect %q: must be one of %q, %q, %q (or omitted)",
				i, l.Expect, ExpectUniversalLink, ExpectOneLink, ExpectCustomScheme)
		}
		if l.FinalHost != "" && l.Expect != ExpectOneLink {
			return fmt.Errorf("links[%d]: finalHost is only meaningful when expect is %q", i, ExpectOneLink)
		}
	}
	if _, err := c.Timeout(0); err != nil {
		return err
	}
	return nil
}
