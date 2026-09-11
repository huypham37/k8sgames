package game

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeCluster struct {
	mu          sync.Mutex
	provisioned string
	deleted     string
	fixed       bool
}

func (f *fakeCluster) Provision(_ context.Context, namespace, manifest string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.provisioned = namespace
	if !strings.Contains(manifest, "image-does-not-exist") {
		return context.Canceled
	}
	return nil
}

func (f *fakeCluster) Inspect(_ context.Context, _ string, args []string) Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.fixed {
		return Result{Output: "nginx:image-does-not-exist"}
	}
	if strings.Contains(strings.Join(args, " "), "availableReplicas") {
		return Result{Output: "2"}
	}
	return Result{Output: "nginx:1.27-alpine"}
}

func (f *fakeCluster) Delete(_ context.Context, namespace string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = namespace
	return nil
}

func (f *fakeCluster) Ping(context.Context) error    { return nil }
func (f *fakeCluster) Cleanup(context.Context) error { return nil }
func (f *fakeCluster) OpenTerminal(context.Context, string, Challenge, TerminalSize) (Terminal, error) {
	return &fakeTerminal{}, nil
}

type fakeTerminal struct{}

func (*fakeTerminal) Read([]byte) (int, error)       { return 0, io.EOF }
func (*fakeTerminal) Write(data []byte) (int, error) { return len(data), nil }
func (*fakeTerminal) Close() error                   { return nil }
func (*fakeTerminal) Resize(TerminalSize) error      { return nil }

func TestSessionLifecycle(t *testing.T) {
	provider := &fakeCluster{}
	manager := NewManager(provider, NewCatalog(), time.Minute, 1)
	created, err := manager.Create(context.Background(), "broken-image")
	if err != nil {
		t.Fatal(err)
	}
	if created.Token == "" || created.Namespace != provider.provisioned {
		t.Fatalf("invalid session: %#v", created)
	}
	if _, err := manager.Get(created.ID, "wrong"); err != ErrUnauthorized {
		t.Fatalf("Get() error = %v, want ErrUnauthorized", err)
	}

	provider.mu.Lock()
	provider.fixed = true
	provider.mu.Unlock()
	result, err := manager.Check(context.Background(), created.ID, created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Completed {
		t.Fatalf("expected completed result: %#v", result)
	}
	if err := manager.Delete(context.Background(), created.ID, created.Token); err != nil {
		t.Fatal(err)
	}
	if provider.deleted != created.Namespace {
		t.Fatalf("deleted %q, want %q", provider.deleted, created.Namespace)
	}
}

func TestSessionLimit(t *testing.T) {
	manager := NewManager(&fakeCluster{}, NewCatalog(), time.Minute, 1)
	if _, err := manager.Create(context.Background(), "broken-image"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Create(context.Background(), "broken-image"); err != ErrFull {
		t.Fatalf("Create() error = %v, want ErrFull", err)
	}
}

func TestCatalog(t *testing.T) {
	catalog := NewCatalog()
	if got := len(catalog.All()); got != 4 {
		t.Fatalf("challenge count = %d, want 4", got)
	}
	for _, challenge := range catalog.All() {
		if challenge.Manifest == "" || len(challenge.Probes) == 0 {
			t.Fatalf("incomplete challenge: %#v", challenge)
		}
		if strings.Contains(challenge.Manifest, "\t") {
			t.Fatalf("challenge %q manifest contains a YAML-invalid tab", challenge.ID)
		}
	}
	broken, err := catalog.Find("broken-image")
	if err != nil {
		t.Fatal(err)
	}
	if len(broken.Probes) != 1 || !strings.Contains(strings.Join(broken.Probes[0].Args, " "), "availableReplicas") {
		t.Fatalf("broken-image must grade its availability objective: %#v", broken.Probes)
	}
}

func TestDeleteExpired(t *testing.T) {
	provider := &fakeCluster{}
	manager := NewManager(provider, NewCatalog(), -time.Second, 1)
	created, err := manager.Create(context.Background(), "broken-image")
	if err != nil {
		t.Fatal(err)
	}
	manager.DeleteExpired(context.Background())
	if provider.deleted != created.Namespace {
		t.Fatalf("deleted %q, want %q", provider.deleted, created.Namespace)
	}
}
