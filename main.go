package main

import "github.com/mubashirtaj/ghostport/cmd"

// version is set at build time via:
//
//	-ldflags "-X main.version={{.Version}}"
var version = "dev"

func main() {
	cmd.Version = version
	cmd.Execute()
}
