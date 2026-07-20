// Package parser turns a raw string into a typed *models.DeepLink.
//
// Three link categories are handled:
//   - Universal Link / App Link  — https:// or http://
//   - Custom scheme              — anything else (myapp://, fb://, …)
package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/space-code/linkctl/internal/models"
)

// Parse parses rawLink into a *models.DeepLink.
//
// Input rules:
//   - Empty / whitespace-only input → error.
//   - Input without "://" is treated as a bare host and https:// is prepended,
//     so "example.com/path" becomes "https://example.com/path".
//   - Scheme is lowercased; host is lowercased.
//   - For duplicate query keys only the first value is kept.
//   - Raw stores the normalised URL (scheme always present), never the
//     original bare input, so callers always have a valid URL string.
func Parse(rawLink string) (*models.DeepLink, error) {
	rawLink = strings.TrimSpace(rawLink)
	if rawLink == "" {
		return nil, fmt.Errorf("deep link cannot be empty")
	}

	// Inject https:// when the caller omits the scheme entirely.
	normalised := rawLink
	if !strings.Contains(rawLink, "://") {
		normalised = "https://" + rawLink
	}

	u, err := url.Parse(normalised)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", normalised, err)
	}

	// url.Parse is extremely permissive — enforce the parts we require.
	if u.Scheme == "" {
		return nil, fmt.Errorf("no scheme in %q", rawLink)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("no host in %q", rawLink)
	}

	// Normalise: scheme and host are always lowercase.
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	return &models.DeepLink{
		Raw:      u.String(),
		Scheme:   u.Scheme,
		Host:     u.Host,
		Path:     u.Path,
		Query:    flattenQuery(u.Query()),
		Fragment: u.Fragment,
		Type:     classify(u.Scheme),
	}, nil
}

// classify assigns a LinkType from the scheme alone.
//
// https / http → LinkTypeUniversalLink.
// The distinction between Universal Link (iOS) and App Link (Android) is
// resolved at validation time once the target platform is known; the parser
// has no project context.
//
// Everything else → LinkTypeCustomScheme.
func classify(scheme string) models.LinkType {
	switch scheme {
	case "https", "http":
		return models.LinkTypeUniversalLink
	default:
		return models.LinkTypeCustomScheme
	}
}

// flattenQuery converts url.Values (map[string][]string) → map[string]string
// by keeping the first value for each key, matching mobile deep link handler
// behaviour which does not expect multi-value params.
// Returns nil when there are no query parameters.
func flattenQuery(q url.Values) map[string]string {
	if len(q) == 0 {
		return nil
	}
	out := make(map[string]string, len(q))
	for k, vs := range q {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out
}
