package cli

import (
	"strings"
	"testing"
)

func TestAPlainHTTPServerInTheConfigIsRefusedBeforeAnyRequest(t *testing.T) {
	path := writeConfig(t, `{"server_url":"http://example.com","current_user_name":"marko"}`)

	_, _, err := open(nil)
	if err == nil {
		t.Fatal("open accepted a plain http server; the token would go out as clear text")
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "plain http") {
		t.Errorf("err = %v, want it to name %s and say why", err, path)
	}
}

func TestALoopbackHTTPServerIsStillAllowed(t *testing.T) {
	writeConfig(t, `{"server_url":"http://127.0.0.1:1","current_user_name":"marko"}`)

	_, _, err := open(nil)
	if err != nil && strings.Contains(err.Error(), "plain http") {
		t.Errorf("open refused a loopback server: %v", err)
	}
}

func TestABrokenDBURLNeverPrintsItsPassword(t *testing.T) {
	for _, dbURL := range []string{
		"postgres://me:s3cret@local host:5432/gator",
		"postgres://me:s3cret@localhost:54x2/gator",
	} {
		_, connectErr := connect(dbURL)
		_, nameErr := localDatabaseName(dbURL)
		for name, err := range map[string]error{"connect": connectErr, "localDatabaseName": nameErr} {
			if err == nil {
				t.Errorf("%s(%q) accepted a broken URL", name, dbURL)
				continue
			}
			if strings.Contains(err.Error(), "s3cret") {
				t.Errorf("%s printed the password: %v", name, err)
			}
		}
	}
}

func TestPasswordInput(t *testing.T) {
	cases := []struct {
		name      string
		given     string
		fromStdin bool
		stdin     string
		want      string
		wantErr   string
	}{
		{"flag alone", "flag-pw", false, "", "flag-pw", ""},
		{"stdin with a trailing newline", "", true, "hunter2\n", "hunter2", ""},
		{"stdin from a Windows pipe", "", true, "hunter2\r\n", "hunter2", ""},
		{"inner spaces survive", "", true, " two words \n", " two words ", ""},
		{"both at once", "flag-pw", true, "hunter2", "", "not both"},
		{"empty stdin", "", true, "", "", "no password"},
		{"only a newline", "", true, "\n", "", "no password"},
		{"far too long", "", true, strings.Repeat("x", maxPasswordInput+1), "", "over"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := passwordInput(c.given, c.fromStdin, strings.NewReader(c.stdin))
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("err = %v, want one containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil || got != c.want {
				t.Errorf("passwordInput = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}
