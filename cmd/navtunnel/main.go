package main

import (
	"fmt"
	"os"

	"github.com/lavp2393/navtunnel/internal/ui"
)

// version se inyecta en build time con:
//
//	-ldflags "-X main.version=<tag>"
//
// GoReleaser / el workflow de GitHub Actions lo setea al tag (v1.2.3).
var version = "dev"

func main() {
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-v" {
			fmt.Println("navtunnel", version)
			return
		}
	}
	ui.NewApp().Run()
}
