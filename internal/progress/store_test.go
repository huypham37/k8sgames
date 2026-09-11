package progress

import (
	"path/filepath"
	"testing"
)

func TestStorePersistsCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "progress.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete("broken-image"); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Has("broken-image") || reopened.Has("scale-up") {
		t.Fatalf("unexpected progress: %#v", reopened.completed)
	}
}
