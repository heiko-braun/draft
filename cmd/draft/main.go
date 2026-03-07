package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/heiko-braun/draft/internal/cli"
)

//go:embed templates/.claude templates/.cursor templates/.principles templates/specs
var templates embed.FS

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli.Execute(templates, version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
