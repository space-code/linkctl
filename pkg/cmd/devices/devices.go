package devices

import (
	"encoding/json"

	"github.com/space-code/linkctl/internal/reporter"
	"github.com/space-code/linkctl/internal/simulator"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	asJSON bool
}

func NewCmdDevices(f *cmdutil.Factory) *cobra.Command {
	opts := options{}

	cmd := &cobra.Command{
		Use:     "devices",
		Short:   "List connected iOS simulators",
		Long:    "List all currently booted iOS simulators.",
		Example: `linkctl devices`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, &opts)
		},
	}

	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "Output results as JSON")

	return cmd
}

type devicesJSON struct {
	IOS   []string        `json:"ios"`
	Tools map[string]bool `json:"tools"`
}

func run(f *cmdutil.Factory, opts *options) error {
	tools := simulator.CheckToolsAvailable()
	var iosDevices []string

	if tools["xcrun"] {
		iosDevices, _ = simulator.ListIOSDevices()
	}

	if opts.asJSON {
		enc := json.NewEncoder(f.IOStreams.Out)
		enc.SetIndent("", " ")
		return enc.Encode(devicesJSON{
			IOS:   iosDevices,
			Tools: tools,
		})
	}

	w := f.IOStreams.Out
	reporter.PrintDeviceList(w, "iOS", iosDevices)
	return nil
}
