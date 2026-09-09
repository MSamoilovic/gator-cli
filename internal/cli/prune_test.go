package cli

import (
	"strings"
	"testing"
	"time"

	"gator-cli/internal/feeds"
)

func TestPruneRejectsExtraArguments(t *testing.T) {
	err := handlerPrune(&state{}, command{Name: "prune", Args: []string{"72h"}})
	if err == nil {
		t.Fatal("a positional argument was accepted, want the usage error")
	}
	if !strings.Contains(err.Error(), "usage: prune") {
		t.Errorf("error = %q, want the usage line", err)
	}
}

func TestPruneRejectsAnUnparsableDuration(t *testing.T) {
	if err := handlerPrune(&state{}, command{Name: "prune", Args: []string{"--older-than", "banana"}}); err == nil {
		t.Fatal("an invalid duration was accepted")
	}
}

func TestPruneIsRegisteredWithoutLogin(t *testing.T) {
	for _, e := range allCommands() {
		if e.name != "prune" {
			continue
		}
		if e.needsLogin() {
			t.Error("prune was registered behind a login; posts are not per-user")
		}
		if e.run == nil {
			t.Error("prune has no run handler")
		}
		return
	}
	t.Fatalf("prune is missing from allCommands(); help and the picker will not show it")
}

func TestPruneDefaultMatchesTheAggLoop(t *testing.T) {
	if feeds.DefaultRetention != 72*time.Hour {
		t.Errorf("DefaultRetention = %s, want the 72h the agg loop prunes with", feeds.DefaultRetention)
	}
}
