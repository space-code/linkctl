package build

import "runtime/debug"

var Version = ""

func init() {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}
