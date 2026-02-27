package root

import (
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
	}

	cmd.AddCommand(versionCmd.NewCmdVersion(f))

	return cmd, nil
}
