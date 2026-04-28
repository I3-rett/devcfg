package main

import (
	"fmt"
	"os"

	"github.com/I3-rett/devcfg/internal/tui"
)

// Version is set at build time via -ldflags "-X main.Version=<tag>".
var Version = "dev"

func main() {
	if err := tui.Run(Version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
