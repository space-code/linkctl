package cmdutil

import "errors"

// ErrChecksFailed signals that the command ran to completion but one or more
// checks did not pass (e.g. a validation FAIL, no registered links found).
//
// Unlike a genuine execution error (bad flags, network failure, malformed
// project) this is an expected, well-formed outcome: the report has already
// been printed, so the top-level error handler must exit with a non-zero
// status without also printing "error: checks failed".
var ErrChecksFailed = errors.New("checks failed")
