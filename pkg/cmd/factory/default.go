package factory

import (
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/space-code/linkctl/pkg/iostreams"
)

func New(appVersion string) *cmdutil.Factory {
	f := &cmdutil.Factory{
		AppVersion:     appVersion,
		ExecutableName: "linkctl",
	}

	f.IOStreams = ioStreams()

	return f
}

func ioStreams() *iostreams.IOStreams {
	io := iostreams.System()
	return io
}
