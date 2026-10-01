//go:build windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func isolateProcessGroup(proc *exec.Cmd) {
	proc.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func terminate(proc *os.Process) error {
	return proc.Kill()
}
