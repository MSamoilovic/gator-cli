package main

import (
	"fmt"
	"os"

	gator "github.com/MSamoilovic/gator-cli"
	"github.com/MSamoilovic/gator-cli/internal/cli"
)

func main() {
	if err := cli.Run(gator.Schema, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
