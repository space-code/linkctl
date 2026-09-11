// Package open implements `linkctl open`, which opens a deep link on a
// booted iOS simulator via `xcrun simctl openurl` — useful for confirming
// a link actually routes into the app after `check-app`/`aasa`/`onelink`
// say the configuration looks correct.
package open

import (
	"encoding/json"
	"fmt"

	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/internal/parser"
	"github.com/space-code/linkctl/internal/reporter"
	"github.com/space-code/linkctl/internal/simulator"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	device string
	asJSON bool
}

func NewCmdOpen(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "open <link>",
		Short: "Open a deep link on a booted iOS simulator",
		Long: `Opens a deep link (Universal Link or custom scheme) on a simulator via
xcrun simctl openurl, to confirm it actually routes into the app rather
than just checking that the configuration looks correct.

Requires Xcode command line tools and a running simulator (defaults to
whichever is currently booted).

Exits with code 0 when the simulator accepted the link, 1 otherwise.`,
		Args: cobra.ExactArgs(1),
		Example: `  linkctl open "myapp://profile/42"
  linkctl open https://example.com/profile/42 --device "iPhone 15"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.device, "device", "d", "", "Target simulator UDID or name (default: booted)")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "Output result as JSON")

	return cmd
}

func run(f *cmdutil.Factory, rawLink string, opts *options) error {
	link, err := parser.Parse(rawLink)
	if err != nil {
		return fmt.Errorf("invalid link: %w", err)
	}

	out, openErr := simulator.OpenOnIOS(opts.device, link.Raw)

	result := &models.SimulationResult{
		Platform: "iOS",
		DeviceID: opts.device,
		Success:  openErr == nil,
		Output:   out,
	}
	if openErr != nil {
		result.Error = openErr.Error()
	}

	if opts.asJSON {
		enc := json.NewEncoder(f.IOStreams.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
	} else {
		reporter.PrintSimulationResult(f.IOStreams.Out, f.IOStreams.ColorScheme(), result)
	}

	if !result.Success {
		return cmdutil.ErrChecksFailed
	}
	return nil
}
