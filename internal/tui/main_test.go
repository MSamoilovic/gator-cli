package tui

import (
	"errors"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	startBrowser = func(string) error { return errors.New("tests never open a browser") }
	writeClipboard = func(string) error { return errors.New("tests never touch the clipboard") }
	os.Exit(m.Run())
}
