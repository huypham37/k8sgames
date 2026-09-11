package progress

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	path      string
	completed map[string]time.Time
}

func Open(path string) (*Store, error) {
	store := &Store{path: path, completed: map[string]time.Time{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &store.completed); err != nil {
		return nil, err
	}
	if store.completed == nil {
		store.completed = map[string]time.Time{}
	}
	return store, nil
}

func DefaultPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "k8sgames", "progress.json"), nil
}

func (s *Store) Has(challenge string) bool {
	_, ok := s.completed[challenge]
	return ok
}

func (s *Store) Complete(challenge string) error {
	if s.Has(challenge) {
		return nil
	}
	s.completed[challenge] = time.Now().UTC()
	data, err := json.MarshalIndent(s.completed, "", "  ")
	if err != nil {
		delete(s.completed, challenge)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		delete(s.completed, challenge)
		return err
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		delete(s.completed, challenge)
		return err
	}
	if err := os.Rename(temporary, s.path); err != nil {
		delete(s.completed, challenge)
		return err
	}
	return nil
}
