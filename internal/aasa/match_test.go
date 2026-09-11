package aasa_test

import (
	"net/url"
	"testing"

	"github.com/space-code/linkctl/internal/aasa"
	"github.com/space-code/linkctl/internal/models"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse test URL %q: %v", raw, err)
	}
	return u
}

func TestMatchDetail_LegacyPaths(t *testing.T) {
	tests := []struct {
		name         string
		paths        []string
		target       string
		wantMatched  bool
		wantExcluded bool
	}{
		{"exact match", []string{"/profile/42"}, "/profile/42", true, false},
		{"star wildcard matches any suffix", []string{"/profile/*"}, "/profile/42/edit", true, false},
		{"star matches nothing", []string{"/profile/*"}, "/profile/", true, false},
		{"question mark matches one char", []string{"/item/?"}, "/item/5", true, false},
		{"question mark rejects two chars", []string{"/item/?"}, "/item/55", false, false},
		{"no match falls through", []string{"/profile/*"}, "/settings", false, false},
		{"NOT prefix excludes", []string{"NOT /admin/*", "/*"}, "/admin/secret", false, true},
		{"NOT prefix does not exclude non-matching path", []string{"NOT /admin/*", "/*"}, "/profile", true, false},
		{"case sensitive by default", []string{"/Profile"}, "/profile", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := models.AASADetail{AppID: "ABCDE12345.com.example.app", Paths: tt.paths}
			res := aasa.MatchDetail(d, mustURL(t, "https://example.com"+tt.target))

			if res.Matched != tt.wantMatched {
				t.Errorf("Matched = %v, want %v", res.Matched, tt.wantMatched)
			}
			if res.Excluded != tt.wantExcluded {
				t.Errorf("Excluded = %v, want %v", res.Excluded, tt.wantExcluded)
			}
		})
	}
}

func TestMatchDetail_Components(t *testing.T) {
	tests := []struct {
		name         string
		components   []models.AASAComponent
		target       string
		wantMatched  bool
		wantExcluded bool
	}{
		{
			name:        "path component matches",
			components:  []models.AASAComponent{{Path: "/profile/*"}},
			target:      "/profile/42",
			wantMatched: true,
		},
		{
			name:         "exclude component wins",
			components:   []models.AASAComponent{{Path: "/private/*", Exclude: true}, {Path: "/*"}},
			target:       "/private/data",
			wantExcluded: true,
		},
		{
			name:        "fragment must match too",
			components:  []models.AASAComponent{{Path: "/share/*", Fragment: "preview"}},
			target:      "/share/42#preview",
			wantMatched: true,
		},
		{
			name:        "fragment mismatch fails component",
			components:  []models.AASAComponent{{Path: "/share/*", Fragment: "preview"}},
			target:      "/share/42#other",
			wantMatched: false,
		},
		{
			name:        "query string glob",
			components:  []models.AASAComponent{{Path: "/promo", Query: "code=*"}},
			target:      "/promo?code=SUMMER",
			wantMatched: true,
		},
		{
			name:        "query object form all keys must match",
			components:  []models.AASAComponent{{Path: "/promo", Query: map[string]any{"a": "1", "b": "*"}}},
			target:      "/promo?a=1&b=anything",
			wantMatched: true,
		},
		{
			name:        "query object form fails when one key mismatches",
			components:  []models.AASAComponent{{Path: "/promo", Query: map[string]any{"a": "1", "b": "2"}}},
			target:      "/promo?a=1&b=3",
			wantMatched: false,
		},
		{
			name:        "case insensitive component",
			components:  []models.AASAComponent{{Path: "/Profile", CaseSensitive: new(bool)}},
			target:      "/profile",
			wantMatched: true,
		},
		{
			name:        "none of several components match",
			components:  []models.AASAComponent{{Path: "/one/*"}, {Path: "/two/*"}},
			target:      "/three",
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := models.AASADetail{AppID: "ABCDE12345.com.example.app", Components: tt.components}
			res := aasa.MatchDetail(d, mustURL(t, "https://example.com"+tt.target))

			if res.Matched != tt.wantMatched {
				t.Errorf("Matched = %v, want %v", res.Matched, tt.wantMatched)
			}
			if res.Excluded != tt.wantExcluded {
				t.Errorf("Excluded = %v, want %v", res.Excluded, tt.wantExcluded)
			}
		})
	}
}

func TestMatchDetail_NoRestrictionMatchesEverything(t *testing.T) {
	d := models.AASADetail{AppID: "ABCDE12345.com.example.app"}
	res := aasa.MatchDetail(d, mustURL(t, "https://example.com/literally/anything"))
	if !res.Matched {
		t.Error("expected an entry with neither components nor paths to match every path")
	}
}

func TestMatchDetail_ComponentsTakePrecedenceOverPaths(t *testing.T) {
	// Per Apple's spec, when both are present, "components" wins and "paths" is ignored.
	d := models.AASADetail{
		AppID:      "ABCDE12345.com.example.app",
		Paths:      []string{"/legacy/*"},
		Components: []models.AASAComponent{{Path: "/modern/*"}},
	}

	if res := aasa.MatchDetail(d, mustURL(t, "https://example.com/legacy/x")); res.Matched {
		t.Error("expected legacy 'paths' to be ignored when 'components' is present")
	}
	if res := aasa.MatchDetail(d, mustURL(t, "https://example.com/modern/x")); !res.Matched {
		t.Error("expected 'components' path to match")
	}
}

func TestMatchAny_StopsAtFirstApplicableDetail(t *testing.T) {
	file := &models.AASAFile{}
	file.AppLinks.Details = []models.AASADetail{
		{AppID: "ABCDE12345.com.example.one", Components: []models.AASAComponent{{Path: "/one/*"}}},
		{AppID: "ABCDE12345.com.example.two", Components: []models.AASAComponent{{Path: "/two/*"}}},
	}

	res := aasa.MatchAny(file, mustURL(t, "https://example.com/two/x"))
	if !res.Matched {
		t.Fatal("expected a match")
	}
	if res.Detail.AppID != "ABCDE12345.com.example.two" {
		t.Errorf("expected match against the second entry, got %q", res.Detail.AppID)
	}
}

func TestMatchAny_NoEntryMatches(t *testing.T) {
	file := &models.AASAFile{}
	file.AppLinks.Details = []models.AASADetail{
		{AppID: "ABCDE12345.com.example.one", Components: []models.AASAComponent{{Path: "/one/*"}}},
	}

	res := aasa.MatchAny(file, mustURL(t, "https://example.com/nope"))
	if res.Matched || res.Excluded {
		t.Error("expected no match and no exclusion")
	}
}
