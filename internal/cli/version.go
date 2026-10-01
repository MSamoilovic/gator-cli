package cli

import (
	"fmt"
	"runtime"
)

func handlerVersion(*state, command) error {
	fmt.Printf("gator %s %s/%s %s\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	return nil
}
