package offline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	QueueFileName = ".gator-queue.jsonl"
	maxQueued     = 1000
)

const (
	opRead       = "read"
	opReadAll    = "read_all"
	opBookmark   = "bookmark"
	opCategorize = "categorize"
)

type change struct {
	Op       string      `json:"op"`
	PostID   uuid.UUID   `json:"post_id,omitempty"`
	PostIDs  []uuid.UUID `json:"post_ids,omitempty"`
	Value    bool        `json:"value,omitempty"`
	URL      string      `json:"url,omitempty"`
	Category string      `json:"category,omitempty"`
	At       time.Time   `json:"at"`
}

type queue struct {
	path    string
	pending []change
}

func loadQueue(path string) *queue {
	q := &queue{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return q
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var c change
		if json.Unmarshal([]byte(line), &c) != nil || c.Op == "" {
			continue
		}
		q.pending = append(q.pending, c)
	}
	return q
}

func (q *queue) add(c change) (int, error) {
	if len(q.pending)+1 > maxQueued {
		dropped := len(q.pending) + 1 - maxQueued
		q.pending = append(q.pending[dropped:], c)
		return dropped, q.rewrite()
	}

	line, err := json.Marshal(c)
	if err != nil {
		return 0, err
	}

	f, err := os.OpenFile(q.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}

	q.pending = append(q.pending, c)
	return 0, nil
}

func (q *queue) sent(n int) error {
	q.pending = q.pending[min(n, len(q.pending)):]
	return q.rewrite()
}

func (q *queue) rewrite() error {
	if len(q.pending) == 0 {
		if err := os.Remove(q.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	tmp, err := os.CreateTemp(filepath.Dir(q.path), QueueFileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	enc := json.NewEncoder(tmp)
	for _, c := range q.pending {
		if err := enc.Encode(c); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), q.path)
}
