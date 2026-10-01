//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

func isolateProcessGroup(proc *exec.Cmd) {
	proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminate(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}
