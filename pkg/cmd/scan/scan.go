package scan

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/space-code/linkctl/internal/appcheck"
	"github.com/space-code/linkctl/internal/reporter"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	asJSON bool
}

func NewCmdScan(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "scan [project-path]",
		Short: "List all deep link patterns registered in an app project",
		Long: `Scans an Xcode or Android project and prints every deep link
pattern the app can handle — Universal Links, App Links, and custom schemes.
 
Platform is detected automatically:
  iOS     — looks for *.xcodeproj in the given directory
  Android — looks for AndroidManifest.xml or build.gradle
 
Exits with code 0 when at least one pattern is found, 1 otherwise.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  deeplink scan
  deeplink scan ./MyApp.xcodeproj
  deeplink scan ./android/app
  deeplink scan --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project := "."
			if len(args) > 0 {
				project = args[0]
			}
			return run(f, project, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "Output results as JSON")

	return cmd
}

func run(f *cmdutil.Factory, project string, opts *options) error {
	result, err := appcheck.ScanProject(project)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	if opts.asJSON {
		enc := json.NewEncoder(f.IOStreams.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
		if len(result.RegisteredLinks) == 0 {
			os.Exit(1)
		}
		return nil
	}

	w := f.IOStreams.Out
	cs := f.IOStreams.ColorScheme()

	// reporter.PrintBanner(w, cs)
	reporter.PrintProjectScan(w, cs, result)

	if len(result.RegisteredLinks) == 0 {
		os.Exit(1)
	}
	return nil
}
