package cmdutil

import "github.com/space-code/linkctl/pkg/iostreams"

type Factory struct {
	AppVersion     string
	ExecutableName string

	IOStreams *iostreams.IOStreams
}
