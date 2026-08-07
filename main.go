package main

import (
	"runtime/debug"

	"github.com/mubashirtaj/ghostport/cmd"
)

var version = "dev"

func main() {
	cmd.Version = resolveVersion()
	cmd.Execute()
}

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}
