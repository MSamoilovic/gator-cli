package cli

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/credentials"
	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

const fakeLocalEnv = "GATOR_FAKE_LOCAL"

func TestMain(m *testing.M) {
	if dir := os.Getenv(fakeLocalEnv); dir != "" {
		os.Exit(fakeLocal(dir, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fakeLocal(dir string, args []string) int {
	record := fmt.Sprintf("%s local=%s args=%s", args[0], os.Getenv(localEnv), strings.Join(args, " "))
	os.WriteFile(filepath.Join(dir, args[0]), []byte(record), 0600)
	if os.Getenv("GATOR_FAKE_FAIL") != "" && args[0] == "serve" {
		return 1
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, shutdownSignals()...)
	if args[0] == "serve" {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v1/healthz", func(w http.ResponseWriter, _ *http.Request) {})
		go http.ListenAndServe(args[2], mux)
	}
	<-stop
	return 0
}

func freeLocalServer(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return "http://" + addr
}

func fakeChildren(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(fakeLocalEnv, dir)
	t.Cleanup(stopLocalServer)
	return dir
}

func running() []*exec.Cmd {
	localServer.mu.Lock()
	defer localServer.mu.Unlock()
	return append([]*exec.Cmd(nil), localServer.procs...)
}

func TestALocalServerIsStartedWhenNobodyAnswersAndStoppedOnQuit(t *testing.T) {
	dir := fakeChildren(t)
	server := freeLocalServer(t)

	if _, err := capture(t, func() error { ensureLocalServer(server, "postgres://localhost/gator"); return nil }); err != nil {
		t.Fatal(err)
	}
	if !answers(server, time.Second) {
		t.Fatal("nothing answers at the server address after ensureLocalServer")
	}
	procs := running()
	if len(procs) != 2 {
		t.Fatalf("%d processes started, want serve and agg", len(procs))
	}
	for name, want := range map[string]string{
		"serve": "serve local=1 args=serve --addr " + strings.TrimPrefix(server, "http://"),
		"agg":   "agg local=1 args=agg " + localAggEvery,
	} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Errorf("%s was started as %q, want %q", name, got, want)
		}
	}

	stopLocalServer()

	if answers(server, 300*time.Millisecond) {
		t.Error("the local server still answers after gator quit")
	}
	for _, p := range procs {
		if p.ProcessState == nil {
			t.Errorf("process %d is still running after gator quit", p.Process.Pid)
		}
	}
}

func TestNoLocalServerIsStartedWhenItIsNotOursToStart(t *testing.T) {
	answering := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(answering.Close)

	cases := []struct {
		name, server, dbURL string
		child               bool
	}{
		{name: "a server on another machine", server: "https://gator.example.com", dbURL: "postgres://localhost/gator"},
		{name: "no database to serve", server: freeLocalServer(t)},
		{name: "a server that already answers", server: answering.URL, dbURL: "postgres://localhost/gator"},
		{name: "gator's own child", server: freeLocalServer(t), dbURL: "postgres://localhost/gator", child: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeChildren(t)
			if tc.child {
				t.Setenv(localEnv, "1")
			}
			ensureLocalServer(tc.server, tc.dbURL)
			if n := len(running()); n != 0 {
				t.Errorf("%d processes started, want none", n)
			}
		})
	}
}

func TestALocalServerThatDiesOnStartLeavesNothingBehind(t *testing.T) {
	fakeChildren(t)
	t.Setenv("GATOR_FAKE_FAIL", "1")
	server := freeLocalServer(t)

	start := time.Now()
	ensureLocalServer(server, "postgres://localhost/gator")
	if took := time.Since(start); took > localReadyWait/2 {
		t.Errorf("gave up after %s; a serve that exited should be noticed at once", took)
	}
	if n := len(running()); n != 0 {
		t.Errorf("%d processes left running after a failed start", n)
	}
}

func TestGatorsOwnChildIgnoresTheServerInTheConfig(t *testing.T) {
	testdb.Open(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(localEnv, "1")
	cfg := config.Config{DBURL: os.Getenv(testdb.EnvURL), ServerURL: "http://127.0.0.1:1", CurrentUserName: "alice"}
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}

	s, closeDB, err := open(nil)
	if err != nil {
		t.Fatalf("open as a child: %v", err)
	}
	defer closeDB()
	if s.Db == nil || s.Cfg.ServerURL != "" {
		t.Errorf("the child opened server %q with Db=%v; it must run on the database", s.Cfg.ServerURL, s.Db != nil)
	}
}

func TestACommandAgainstThisMachinesServerStartsItAndStopsItWhenDone(t *testing.T) {
	fakeChildren(t)
	server := freeLocalServer(t)
	cfg := config.Config{DBURL: "postgres://localhost/gator", ServerURL: server, CurrentUserName: "alice"}
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}
	if err := credentials.Save(credentials.Credentials{ServerURL: server, Token: "gat_alice"}); err != nil {
		t.Fatal(err)
	}

	var seen bool
	saved := spawnLocal
	spawnLocal = func(args ...string) (*exec.Cmd, error) {
		seen = true
		return saved(args...)
	}
	t.Cleanup(func() { spawnLocal = saved })

	capture(t, func() error { return Run(nil, []string{"following"}) })

	if !seen {
		t.Fatal("a command against a server on this machine did not start it")
	}
	if n := len(running()); n != 0 {
		t.Errorf("%d processes still running after the command returned", n)
	}
	if answers(server, 300*time.Millisecond) {
		t.Error("the local server still answers after gator quit")
	}
}
