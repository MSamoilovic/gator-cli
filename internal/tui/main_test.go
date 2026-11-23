package tui

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	time.Local = time.UTC
	startBrowser = func(string) error { return errors.New("tests never open a browser") }
	writeClipboard = func(string) error { return errors.New("tests never touch the clipboard") }
	os.Exit(m.Run())
}
