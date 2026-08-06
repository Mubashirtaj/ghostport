package main

import (
	"runtime/debug"

	"github.com/mubashirtaj/ghostport/cmd"
)

// version is set at build time via:
//
//	-ldflags "-X main.version={{.Version}}"
var version = "dev"

func main() {
	cmd.Version = resolveVersion()
	cmd.Execute()
}

// resolveVersion falls back to the module version Go embeds automatically
// when installed via `go install .../ghostport@version`, since that path
// doesn't run GoReleaser's ldflags.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}
