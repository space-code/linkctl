package cache

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/space-code/linkctl/internal/simulator"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type Options struct {
	Factory *cmdutil.Factory

	Platform   string
	Device     string
	Package    string
	JSONOutput bool
}

type ResetResult struct {
	Success  bool   `json:"success"`
	Platform string `json:"platform"`
	Device   string `json:"device,omitempty"`
	Message  string `json:"message"`
	Error    string `json:"error,omitempty"`
}

func NewCmdCacheReset(f *cmdutil.Factory) *cobra.Command {
	opts := &Options{
		Factory: f,
	}

	cmd := &cobra.Command{
		Use:   "cache-reset",
		Short: "Reset Universal Links (iOS) or App Links (Android) cache on simulator/emulator",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCacheReset(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Platform, "platform", "p", "ios", "Platform to reset cache for ios")
	cmd.Flags().StringVarP(&opts.Device, "device", "d", "", "Target simulator/emulator ID or name")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Output result as JSON")

	_ = cmd.MarkFlagRequired("platform")

	return cmd
}

func runCacheReset(opts *Options) error {
	platform := strings.ToLower(strings.TrimSpace(opts.Platform))
	result := &ResetResult{
		Platform: platform,
		Device:   opts.Device,
	}

	switch platform {
	case "ios":
		err := simulator.ResetIOSUniversalLinksCache(opts.Device)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			result.Message = "Failed to reset iOS Universal Links cache"
		} else {
			result.Success = true
			result.Message = "Successfully reset iOS Universal Links cache"
		}
	default:
		err := fmt.Errorf("unsupported platform '%s': must be 'ios'", opts.Platform)
		result.Success = false
		result.Error = err.Error()
		result.Message = "Validation error"
		_ = printResult(opts, result)
		return err
	}

	return printResult(opts, result)
}

func printResult(opts *Options, result *ResetResult) error {
	ios := opts.Factory.IOStreams

	if opts.JSONOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		fmt.Fprintln(ios.Out, string(data))
		return nil
	}

	cs := ios.ColorScheme()

	if result.Success {
		fmt.Fprintf(ios.Out, "✔ %s\n", cs.Green(result.Message))
		if result.Device != "" {
			fmt.Fprintf(ios.Out, "  └─ Target Device: %s\n", result.Device)
		}
	} else {
		fmt.Fprintf(ios.Out, "✖ %s\n", cs.Red(result.Message))
		if result.Error != "" {
			fmt.Fprintf(ios.Out, "  └─ Details: %s\n", result.Error)
		}
	}

	return nil
}
