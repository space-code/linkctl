package resolve

import (
	"fmt"

	"github.com/space-code/linkctl/internal/models"
)

// Check turns a Trace into the PASS/FAIL/WARN/INFO vocabulary used
// everywhere else in linkctl.
func Check(t *Trace) []models.ValidationResult {
	results := make([]models.ValidationResult, 0, len(t.Issues)+2)
	results = append(results, t.Issues...)

	hasFailure := false
	for _, issue := range t.Issues {
		if issue.Status == models.StatusFail {
			hasFailure = true
		}
	}

	redirectHops := 0
	for _, h := range t.Hops {
		if h.Kind == HopKindHTTP || h.Kind == HopKindMetaRefresh || h.Kind == HopKindJS {
			redirectHops++
		}
	}

	if !hasFailure {
		status := models.StatusPass
		if t.Truncated {
			status = models.StatusWarning
		}
		results = append(results, models.ValidationResult{
			Check:   "Redirect Chain",
			Status:  status,
			Message: fmt.Sprintf("%d redirect(s) followed", redirectHops),
		})
	}

	if t.Final != "" {
		var lastKind HopKind
		if len(t.Hops) > 0 {
			lastKind = t.Hops[len(t.Hops)-1].Kind
		}

		switch lastKind {
		case HopKindCustomScheme:
			results = append(results, models.ValidationResult{
				Check:   "Final Destination",
				Status:  models.StatusPass,
				Message: "resolves to a custom-scheme deep link",
				Detail:  t.Final,
			})
		case HopKindFinal:
			status := models.StatusInfo
			if t.FinalStatus >= 400 {
				status = models.StatusFail
			}
			results = append(results, models.ValidationResult{
				Check:   "Final Destination",
				Status:  status,
				Message: fmt.Sprintf("HTTP %d, no deep link scheme reached", t.FinalStatus),
				Detail:  t.Final,
			})
		}
	}

	return results
}
