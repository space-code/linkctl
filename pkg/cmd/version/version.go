package version

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/space-code/linkctl/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdVersion(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "version",
		Short:  "Show linkctl version information",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprint(f.IOStreams.Out, cmd.Root().Annotations["versionInfo"])
		},
	}

	return cmd
}

func Format(version string) string {
	version = strings.TrimPrefix(version, "v")

	return fmt.Sprintf("linkctl version %s\n%s\n", version, changelogURL(version))
}

func changelogURL(version string) string {
	path := "https://github.com/space-code/linkctl"
	r := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[\w.]+)?$`)
	if !r.MatchString(version) {
		return fmt.Sprintf("%s/releases/latest", path)
	}

	url := fmt.Sprintf("%s/releases/tag/v%s", path, strings.TrimPrefix(version, "v"))
	return url
}
