package validate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/space-code/linkctl/internal/reporter"
	"github.com/space-code/linkctl/internal/validator"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	asJSON bool
}

func NewCmdValidate(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "validate <link>",
		Short: "Validate server-side deep link configuration (AASA)",
		Long: `Validates the server-side configuration for a given deep link.
Fetches apple-app-site-association (iOS) over the network
and verifies domain health, SSL certificate, HTTP headers, and JSON structure.

Exits with code 0 when validation passes without errors, 1 otherwise.`,
		Args: cobra.ExactArgs(1),
		Example: `  linkctl validate https://example.com/profile
  linkctl validate https://example.com/app/path --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			link := args[0]
			return run(f, link, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "Output results as JSON")

	return cmd
}

func run(f *cmdutil.Factory, link string, opts *options) error {
	result, err := validator.ValidateDeepLink(link)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if opts.asJSON {
		enc := json.NewEncoder(f.IOStreams.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
		if result.HasErrors() {
			os.Exit(1)
		}
		return nil
	}

	w := f.IOStreams.Out
	cs := f.IOStreams.ColorScheme()

	reporter.PrintValidationResult(w, cs, result)

	if result.HasErrors() {
		os.Exit(1)
	}
	return nil
}
