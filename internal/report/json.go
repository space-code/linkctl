package report

import (
	"encoding/json"
	"io"
)

// WriteJSON marshals r as indented JSON, matching the json.NewEncoder +
// SetIndent("", "  ") convention used by every other --json command in
// this codebase.
func WriteJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
