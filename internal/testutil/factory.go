// Package testutil holds helpers shared by pkg/cmd/*/*_test.go files.
// It is a regular (non-_test.go) package so it can be imported from the
// external test packages (package x_test) used throughout the repo.
package testutil

import (
	"bytes"
	"testing"

	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/space-code/linkctl/pkg/iostreams"
)

// NewFactory returns a *cmdutil.Factory wired to an in-memory IOStreams,
// plus the stdout buffer commands under test write to. Colour output is
// disabled so assertions on printed text don't need to strip ANSI codes.
func NewFactory(t *testing.T) (*cmdutil.Factory, *bytes.Buffer) {
	t.Helper()

	ios, _, stdout, _ := iostreams.Test()

	f := &cmdutil.Factory{
		AppVersion:     "1.0.0",
		ExecutableName: "linkctl",
		IOStreams:      ios,
	}

	return f, stdout
}
