package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const fileName = ".gatorcredentials.json"

var ErrNoToken = errors.New("no saved token for that server")

type Credentials struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, fileName), nil
}

func Load(serverURL string) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoToken
	}
	if err != nil {
		return "", fmt.Errorf("reading credentials: %w", err)
	}

	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}
	if c.ServerURL != serverURL || c.Token == "" {
		return "", ErrNoToken
	}
	return c.Token, nil
}

func Save(c Credentials) error {
	path, err := Path()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), fileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(c); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func Delete() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
