package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const (
	superviseLogFile   = "gator-agg.log"
	superviseMinBackff = 1 * time.Second
	superviseMaxBackff = 60 * time.Second
	superviseHealthy   = 30 * time.Second
)

// handlerSupervise keeps `gator agg` running, restarting it with exponential
// backoff whenever it crashes. It logs to both the terminal and a log file,
// and shuts down cleanly on SIGINT/SIGTERM.
func handlerSupervise(_ *state, cmd command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: supervise <time_between_reqs>")
	}
	interval := cmd.Args[0]
	if _, err := time.ParseDuration(interval); err != nil {
		return fmt.Errorf("invalid duration: %v", err)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate own executable: %v", err)
	}

	logFile, err := os.OpenFile(superviseLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %v", err)
	}
	defer logFile.Close()

	out := io.MultiWriter(os.Stdout, logFile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backoff := superviseMinBackff
	for {
		if ctx.Err() != nil {
			return nil
		}

		fmt.Fprintf(out, "[supervisor] starting: %s agg %s\n", self, interval)
		start := time.Now()

		proc := exec.Command(self, "agg", interval)
		proc.Stdout = out
		proc.Stderr = out

		proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := proc.Start(); err != nil {
			fmt.Fprintf(out, "[supervisor] failed to start: %v\n", err)
		} else {
			done := make(chan error, 1)
			go func() { done <- proc.Wait() }()

			select {
			case <-ctx.Done():
				fmt.Fprintln(out, "[supervisor] signal received, stopping child...")
				proc.Process.Signal(syscall.SIGTERM)
				<-done
				fmt.Fprintln(out, "[supervisor] shut down cleanly")
				return nil
			case werr := <-done:
				if werr == nil {
					fmt.Fprintln(out, "[supervisor] child exited cleanly, stopping")
					return nil
				}
				fmt.Fprintf(out, "[supervisor] child crashed: %v\n", werr)
			}
		}

		// Ako je dete radilo dovoljno dugo, tretiramo restart kao svez pocetak.
		if time.Since(start) > superviseHealthy {
			backoff = superviseMinBackff
		}

		fmt.Fprintf(out, "[supervisor] restarting in %s\n", backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > superviseMaxBackff {
			backoff = superviseMaxBackff
		}
	}
}
