package main

import "github.com/pv3dev/pv3/cmd/pv3/local"

// Set by -ldflags at build time
var version = "dev"

func main() {
	local.Execute(version)
}
