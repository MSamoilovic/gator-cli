package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/MSamoilovic/gator-cli/internal/cli"
)

//go:embed sql/schema
var schema embed.FS

func main() {
	if err := cli.Run(schema, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
