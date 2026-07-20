package validator

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxAASASizeBytes = 128 * 1024 // 128 KB limit according to Apple spec

var appIDRegex = regexp.MustCompile(`^[A-Z0-9]{10}\.[a-zA-Z0-9_.-]+$`)

type Issue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type Result struct {
	URL        string  `json:"url"`
	Domain     string  `json:"domain"`
	FileType   string  `json:"file_type,omitempty"`
	ConfigURL  string  `json:"config_url,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	Issues     []Issue `json:"issues"`
	Valid      bool    `json:"valid"`
}

func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Level == "error" {
			return true
		}
	}
	return false
}

func ValidateDeepLink(rawURL string) (*Result, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL: %s", rawURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &Result{
			URL:    rawURL,
			Valid:  true,
			Issues: []Issue{{Level: "info", Message: "Custom scheme detected, no server-side validation needed"}},
		}, nil
	}

	result := &Result{
		URL:    rawURL,
		Domain: parsedURL.Hostname(),
		Valid:  true,
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}

	scheme := parsedURL.Scheme
	host := parsedURL.Host

	aasaURL := fmt.Sprintf("%s://%s/.well-known/apple-app-site-association", scheme, host)
	checkAASA(client, aasaURL, result)

	result.Valid = !result.HasErrors()
	return result, nil
}

func checkAASA(client *http.Client, targetURL string, res *Result) {
	resp, err := client.Get(targetURL)
	if err != nil {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: fmt.Sprintf("Failed to fetch AASA: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: fmt.Sprintf("AASA endpoint redirected with status %d (Apple requires direct 200 OK without redirects)", resp.StatusCode),
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: fmt.Sprintf("AASA HTTP status %d (expected 200)", resp.StatusCode),
		})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		res.Issues = append(res.Issues, Issue{
			Level:   "warning",
			Message: fmt.Sprintf("AASA Content-Type is '%s' (expected 'application/json')", contentType),
		})
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAASASizeBytes+1))
	if err != nil {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: "Failed to read AASA response body",
		})
		return
	}

	if len(body) > maxAASASizeBytes {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: fmt.Sprintf("AASA file size exceeds the 128 KB limit (%d bytes)", len(body)),
		})
		return
	}

	var aasaContent map[string]interface{}
	if err := json.Unmarshal(body, &aasaContent); err != nil {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: "AASA is not valid JSON",
		})
		return
	}

	applinksRaw, ok := aasaContent["applinks"]
	if !ok {
		res.Issues = append(res.Issues, Issue{
			Level:   "warning",
			Message: "AASA JSON does not contain 'applinks' key",
		})
		return
	}

	applinks, ok := applinksRaw.(map[string]interface{})
	if !ok {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: "'applinks' section must be a JSON object",
		})
		return
	}

	validateAASAApplinksDetails(applinks, res)
}

func validateAASAApplinksDetails(applinks map[string]interface{}, res *Result) {
	detailsRaw, ok := applinks["details"]
	if !ok {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: "'applinks' object is missing 'details' field",
		})
		return
	}

	var detailsList []map[string]interface{}

	switch v := detailsRaw.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				detailsList = append(detailsList, m)
			}
		}
	case map[string]interface{}:
		for appID, data := range v {
			if m, ok := data.(map[string]interface{}); ok {
				m["appID"] = appID
				detailsList = append(detailsList, m)
			}
		}
	default:
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: "'details' field in AASA must be an array or object",
		})
		return
	}

	if len(detailsList) == 0 {
		res.Issues = append(res.Issues, Issue{
			Level:   "warning",
			Message: "'details' array in AASA is empty",
		})
		return
	}

	for i, entry := range detailsList {
		validateAASAEntry(entry, i, res)
	}
}

func validateAASAEntry(entry map[string]interface{}, index int, res *Result) {
	var appIDs []string
	if appIDStr, ok := entry["appID"].(string); ok {
		appIDs = append(appIDs, appIDStr)
	}
	if appIDsSlice, ok := entry["appIDs"].([]interface{}); ok {
		for _, id := range appIDsSlice {
			if idStr, ok := id.(string); ok {
				appIDs = append(appIDs, idStr)
			}
		}
	}

	if len(appIDs) == 0 {
		res.Issues = append(res.Issues, Issue{
			Level:   "error",
			Message: fmt.Sprintf("AASA details entry #%d does not specify 'appID' or 'appIDs'", index),
		})
	} else {
		for _, id := range appIDs {
			if !appIDRegex.MatchString(id) {
				res.Issues = append(res.Issues, Issue{
					Level:   "warning",
					Message: fmt.Sprintf("App ID '%s' in entry #%d does not match standard <TeamID>.<BundleID> pattern", id, index),
				})
			}
		}
	}

	_, hasComponents := entry["components"]
	_, hasPaths := entry["paths"]

	if !hasComponents && !hasPaths {
		res.Issues = append(res.Issues, Issue{
			Level:   "warning",
			Message: fmt.Sprintf("AASA details entry #%d has neither 'components' nor 'paths' defined", index),
		})
	}
}
