package cli

import (
	"context"
	"fmt"
	"runtime"
)

func handlerVersion(context.Context, *state, command) error {
	fmt.Printf("gator %s %s/%s %s\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	return nil
}
