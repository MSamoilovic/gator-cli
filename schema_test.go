package gator

import (
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
)

func TestTheEmbedCarriesEveryMigrationOnDisk(t *testing.T) {
	onDisk := map[string]bool{}
	entries, err := os.ReadDir("sql/schema")
	if err != nil {
		t.Fatalf("read sql/schema: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			onDisk[e.Name()] = true
		}
	}
	if len(onDisk) == 0 {
		t.Fatal("no migrations on disk, so this test proves nothing")
	}

	embedded := map[string]bool{}
	err = fs.WalkDir(Schema, "sql/schema", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".sql") {
			embedded[path.Base(p)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the embedded schema: %v", err)
	}

	for name := range onDisk {
		if !embedded[name] {
			t.Errorf("%s is not in the binary, so gator init and gator migrate will not apply it", name)
		}
	}
	for name := range embedded {
		if !onDisk[name] {
			t.Errorf("%s is embedded but no longer on disk", name)
		}
	}
}
