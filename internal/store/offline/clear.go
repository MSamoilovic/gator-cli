package offline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func Clear(dir string) (int, error) {
	lost := len(loadQueue(filepath.Join(dir, QueueFileName)).pending)
	for _, name := range []string{QueueFileName, FileName} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return lost, err
		}
	}
	return lost, nil
}

func ClearUnlessOwnedBy(dir, server, user string) (int, error) {
	var snap snapshot
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err == nil && json.Unmarshal(data, &snap) == nil && snap.Server == server && snap.Owner == user {
		return 0, nil
	}
	return Clear(dir)
}
