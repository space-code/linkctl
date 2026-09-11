package aasa_test

import (
	"testing"

	"github.com/space-code/linkctl/internal/aasa"
)

func TestParse_ArrayForm(t *testing.T) {
	body := []byte(`{
		"applinks": {
			"apps": [],
			"details": [
				{"appID": "ABCDE12345.com.example.app", "paths": ["/profile/*"]}
			]
		}
	}`)

	file, err := aasa.Parse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(file.AppLinks.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(file.AppLinks.Details))
	}
	if file.AppLinks.Details[0].AppID != "ABCDE12345.com.example.app" {
		t.Errorf("unexpected appID: %q", file.AppLinks.Details[0].AppID)
	}
}

func TestParse_LegacyObjectForm(t *testing.T) {
	body := []byte(`{
		"applinks": {
			"details": {
				"ABCDE12345.com.example.app": {"paths": ["/profile/*"]}
			}
		}
	}`)

	file, err := aasa.Parse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(file.AppLinks.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(file.AppLinks.Details))
	}
	if file.AppLinks.Details[0].AppID != "ABCDE12345.com.example.app" {
		t.Errorf("expected the map key to become the appID, got %q", file.AppLinks.Details[0].AppID)
	}
}

func TestParse_AppIDsPlural(t *testing.T) {
	body := []byte(`{
		"applinks": {
			"details": [
				{"appIDs": ["ABCDE12345.com.example.one", "ABCDE12345.com.example.two"], "paths": ["*"]}
			]
		}
	}`)

	file, err := aasa.Parse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := file.AppLinks.Details[0].AllAppIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 app ids, got %d: %v", len(ids), ids)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := aasa.Parse([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestParse_DetailsWrongType(t *testing.T) {
	_, err := aasa.Parse([]byte(`{"applinks": {"details": "not-an-array-or-object"}}`))
	if err == nil {
		t.Fatal("expected an error when 'details' is neither array nor object")
	}
}

func TestParse_MissingDetails(t *testing.T) {
	file, err := aasa.Parse([]byte(`{"applinks": {"apps": []}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(file.AppLinks.Details) != 0 {
		t.Errorf("expected no details, got %d", len(file.AppLinks.Details))
	}
}

func TestParse_ComponentsWithObjectQuery(t *testing.T) {
	body := []byte(`{
		"applinks": {
			"details": [
				{
					"appID": "ABCDE12345.com.example.app",
					"components": [
						{"/": "/promo", "?": {"code": "*"}, "caseSensitive": false}
					]
				}
			]
		}
	}`)

	file, err := aasa.Parse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := file.AppLinks.Details[0].Components[0]
	if c.CaseSensitive == nil || *c.CaseSensitive != false {
		t.Errorf("expected caseSensitive=false to be parsed, got %+v", c.CaseSensitive)
	}
	if _, ok := c.Query.(map[string]any); !ok {
		t.Errorf("expected Query to decode as an object, got %T", c.Query)
	}
}
