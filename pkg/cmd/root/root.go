package root

import (
	aasaCmd "github.com/space-code/linkctl/pkg/cmd/aasa"
	cacheCmd "github.com/space-code/linkctl/pkg/cmd/cache"
	checkCmd "github.com/space-code/linkctl/pkg/cmd/check"
	ciCmd "github.com/space-code/linkctl/pkg/cmd/ci"
	devicesCmd "github.com/space-code/linkctl/pkg/cmd/devices"
	onelinkCmd "github.com/space-code/linkctl/pkg/cmd/onelink"
	openCmd "github.com/space-code/linkctl/pkg/cmd/open"
	resolveCmd "github.com/space-code/linkctl/pkg/cmd/resolve"
	scanCmd "github.com/space-code/linkctl/pkg/cmd/scan"
	validateCmd "github.com/space-code/linkctl/pkg/cmd/validate"
	versionCmd "github.com/space-code/linkctl/pkg/cmd/version"
	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdRoot(f *cmdutil.Factory, appVersion string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "linkctl",
		Short: "Mobile Deep Link Debugger",
		Long:  "linkctl - debug universal links, deeplinks, and app links.",
		Annotations: map[string]string{
			"versionInfo": versionCmd.Format(appVersion),
		},
		// Errors and usage are handled once, centrally, in internal/cmd.Main —
		// letting cobra also print them would duplicate the message and dump
		// full usage text on every check failure, which is unreadable in CI logs.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(versionCmd.NewCmdVersion(f))
	cmd.AddCommand(devicesCmd.NewCmdDevices(f))
	cmd.AddCommand(checkCmd.NewCmdCheck(f))
	cmd.AddCommand(scanCmd.NewCmdScan(f))
	cmd.AddCommand(validateCmd.NewCmdValidate(f))
	cmd.AddCommand(cacheCmd.NewCmdCacheReset(f))
	cmd.AddCommand(aasaCmd.NewCmdAASA(f))
	cmd.AddCommand(resolveCmd.NewCmdResolve(f))
	cmd.AddCommand(onelinkCmd.NewCmdOneLink(f))
	cmd.AddCommand(openCmd.NewCmdOpen(f))
	cmd.AddCommand(ciCmd.NewCmdCI(f))

	return cmd, nil
}
