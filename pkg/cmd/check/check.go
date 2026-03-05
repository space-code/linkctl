package check

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/space-code/linkctl/internal/appcheck"
	"github.com/space-code/linkctl/internal/parser"
	"github.com/space-code/linkctl/internal/reporter"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type options struct {
	project       string
	target        string
	configuration string
	asJSON        bool
}

func NewCmdCheck(f *cmdutil.Factory) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:     "check-app <link>",
		Short:   "Validate a deep link against your app project configuration",
		Long:    "Parses your app project and checks that it is correctly configured to handle the given deep link — without requiring a running simulator.",
		Example: `linkctl check-app "myapp://profile" --project ./MyApp.xcodeproj`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", ".", "Path to .xcodeproj or Android project directory")
	cmd.Flags().StringVarP(&opts.target, "target", "t", "", "Xcode target name to check (default: all targets)")
	cmd.Flags().StringVarP(&opts.configuration, "configuration", "c", "", "Xcode build configuration name (e.g. Debug, Release)")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "Output results as JSON")

	return cmd
}

func run(f *cmdutil.Factory, rawLink string, opts *options) error {
	link, err := parser.Parse(rawLink)
	if err != nil {
		return fmt.Errorf("invalid link: %w", err)
	}

	report, err := appcheck.CheckApp(opts.project, link, opts.target, opts.configuration)
	if err != nil {
		return fmt.Errorf("check-app: %w", err)
	}

	if opts.asJSON {
		enc := json.NewEncoder(f.IOStreams.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
		if !report.Summary.OK {
			os.Exit(1)
		}
		return nil
	}

	w := f.IOStreams.Out
	cs := f.IOStreams.ColorScheme()

	// reporter.PrintBanner(w, cs)
	reporter.PrintLinkInfo(w, cs, link)
	reporter.PrintAppCheckReport(w, cs, report)

	fmt.Println(report.Checks)

	if !report.Summary.OK {
		os.Exit(1)
	}
	return nil
}
