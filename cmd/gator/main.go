package main

import (
	"fmt"
	"os"

	gator "github.com/MSamoilovic/gator-cli"
	"github.com/MSamoilovic/gator-cli/internal/cli"
	"github.com/MSamoilovic/gator-cli/internal/text"
)

func main() {
	if err := cli.Run(gator.Schema, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, text.Printable(err.Error()))
		os.Exit(1)
	}
}
