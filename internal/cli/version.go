package cli

import (
	"context"
	"fmt"
	"runtime"
)

func handlerVersion(context.Context, *state, command) error {
	fmt.Printf("gator %s %s/%s %s\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	if defaultServer != "" {
		fmt.Printf("server %s (built in; gator init --db-url ... to use a local database instead)\n", defaultServer)
	}
	return nil
}
