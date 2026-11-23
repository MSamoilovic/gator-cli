package cli

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/config"
)

const (
	localEnv       = "GATOR_LOCAL"
	localAggEvery  = "15m"
	localLogName   = ".gator-local.log"
	localProbeWait = 500 * time.Millisecond
	localReadyWait = 10 * time.Second
	localStopWait  = 5 * time.Second
)

var spawnLocal = func(args ...string) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	proc := exec.Command(self, args...)
	proc.Env = append(os.Environ(), localEnv+"=1")
	return proc, nil
}

var localServer struct {
	mu    sync.Mutex
	procs []*exec.Cmd
	done  []chan struct{}
}

func runsLocally() bool {
	return os.Getenv(localEnv) != ""
}

func ensureLocalServerFor(server string) {
	cfg, err := config.Read()
	if err != nil {
		return
	}
	ensureLocalServer(server, cfg.DBURL)
}

func ensureLocalServer(server, dbURL string) {
	if server == "" || dbURL == "" || runsLocally() {
		return
	}
	u, err := url.Parse(server)
	if err != nil || u.Scheme != "http" || !loopbackHost(u.Hostname()) {
		return
	}
	if answers(server, localProbeWait) {
		return
	}

	localServer.mu.Lock()
	defer localServer.mu.Unlock()
	if len(localServer.procs) > 0 {
		return
	}

	addr := u.Host
	if u.Port() == "" {
		addr = net.JoinHostPort(u.Hostname(), "80")
	}

	logPath, logFile := openLocalLog()
	if logFile != nil {
		defer logFile.Close()
	}

	for _, args := range [][]string{{"serve", "--addr", addr}, {"agg", localAggEvery}} {
		if err := startLocal(logFile, args...); err != nil {
			stopLocked()
			fmt.Fprintf(os.Stderr, "warning: could not start a local server for %s: %v\n", server, err)
			return
		}
	}

	deadline := time.Now().Add(localReadyWait)
	for !answers(server, localProbeWait) {
		if time.Now().After(deadline) || exited() {
			stopLocked()
			fmt.Fprintf(os.Stderr, "warning: the local server for %s did not start; see %s\n", server, logPath)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "Started a local server for %s; it stops when gator quits (log: %s)\n", server, logPath)
}

func openLocalLog() (string, *os.File) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}
	path := filepath.Join(home, localLogName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return path, nil
	}
	return path, f
}

func startLocal(log *os.File, args ...string) error {
	proc, err := spawnLocal(args...)
	if err != nil {
		return err
	}
	if log != nil {
		proc.Stdout, proc.Stderr = log, log
	}
	if err := proc.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()
	localServer.procs = append(localServer.procs, proc)
	localServer.done = append(localServer.done, done)
	return nil
}

func exited() bool {
	for _, done := range localServer.done {
		select {
		case <-done:
			return true
		default:
		}
	}
	return false
}

func answers(server string, wait time.Duration) bool {
	client := http.Client{Timeout: wait}
	res, err := client.Get(server + "/v1/healthz")
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func stopLocalServer() {
	localServer.mu.Lock()
	defer localServer.mu.Unlock()
	stopLocked()
}

func stopLocked() {
	for _, proc := range localServer.procs {
		terminate(proc.Process)
	}
	timer := time.NewTimer(localStopWait)
	defer timer.Stop()
	expired := false
	for i, done := range localServer.done {
		if !expired {
			select {
			case <-done:
				continue
			case <-timer.C:
				expired = true
			}
		}
		localServer.procs[i].Process.Kill()
		<-done
	}
	localServer.procs, localServer.done = nil, nil
}
