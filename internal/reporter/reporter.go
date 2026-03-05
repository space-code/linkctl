package reporter

import (
	"fmt"
	"io"

	"github.com/space-code/linkctl/pkg/iostreams"
)

func PrintBanner(w io.Writer) {
	fmt.Println(w, "Debugger")
}

func PrintDeviceList(w io.Writer, cs *iostreams.ColorScheme, platform string, devices []string) {
	if len(devices) == 0 {
		fmt.Fprintf(w, "  %s  No %s devices found (booted / connected)\n\n", "⚠️", platform)
		return
	}

	fmt.Fprintf(w, "%s\n", cs.Bold(platform))
	for _, d := range devices {
		fmt.Fprintf(w, "%s %s\n", cs.Muted("•"), d)
	}
	fmt.Fprintln(w)
}
