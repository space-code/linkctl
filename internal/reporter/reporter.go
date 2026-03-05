package reporter

import (
	"fmt"
	"io"
)

func PrintBanner(w io.Writer) {
	fmt.Println(w, "Debugger")
}

func PrintDeviceList(w io.Writer, platform string, devices []string) {
	if len(devices) == 0 {
		fmt.Fprintf(w, " No %s devices found (booted / connected)\n\n", platform)
		return
	}

	fmt.Fprintf(w, "%s\n", platform)

	for _, d := range devices {
		fmt.Fprintf(w, "%s %s\n", "*", d)
	}
	fmt.Fprintln(w)
}
