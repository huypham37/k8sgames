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
	results     []Result
}

func (f *fakeCluster) Provision(_ context.Context, namespace string, challenge Challenge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.provisioned = namespace
	if !strings.Contains(challenge.Manifest, "image-does-not-exist") {
		return context.Canceled
	}
	return nil
}

func (f *fakeCluster) Inspect(_ context.Context, _ string, args []string) Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.results) > 0 {
		result := f.results[0]
		f.results = f.results[1:]
		return result
	}
	if !f.fixed {
		return Result{Output: "nginx:image-does-not-exist"}
	}
	if strings.Contains(strings.Join(args, " "), "availableReplicas") {
		return Result{Output: "2"}
	}
	return Result{Output: "nginx:1.27-alpine"}
}

func TestGradeComparisons(t *testing.T) {
	provider := &fakeCluster{results: []Result{{Output: "list get"}, {Output: "3"}}}
	manager := NewManager(provider, NewCatalog(), time.Minute, 1)
	session := Session{Challenge: Challenge{Probes: []Probe{
		unorderedProbe([]string{"one"}, []string{"get", "list"}, "wrong set"),
		atLeastProbe([]string{"two"}, 2, "too small"),
	}}}
	completed, progress := manager.grade(context.Background(), &session)
	if !completed || progress != "Challenge complete." {
		t.Fatalf("grade() = %v, %q", completed, progress)
	}
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
	expected := []string{
		"first-pod", "scale-up", "scheduling", "broken-image", "crash-loop", "self-healing",
		"daemonset", "batch-workloads", "rolling-rollback",
		"service-discovery", "service-selector", "ingress-tls", "network-segmentation", "dns-debugging",
		"configmaps", "secrets", "persistent-storage", "statefulset",
		"resource-limits", "production-readiness", "rbac-fortress", "outage-resilience", "three-tier", "black-friday", "full-production",
	}
	if got := len(catalog.All()); got != len(expected) {
		t.Fatalf("challenge count = %d, want %d", got, len(expected))
	}
	seen := map[string]bool{}
	for index, challenge := range catalog.All() {
		if challenge.ID != expected[index] {
			t.Fatalf("challenge %d = %q, want %q", index, challenge.ID, expected[index])
		}
		if challenge.ID == "" || challenge.Title == "" || challenge.Chapter == "" || challenge.Objective == "" || challenge.Hint == "" || challenge.Manifest == "" || len(challenge.Probes) == 0 {
			t.Fatalf("incomplete challenge: %#v", challenge)
		}
		if seen[challenge.ID] {
			t.Fatalf("duplicate challenge ID %q", challenge.ID)
		}
		seen[challenge.ID] = true
		if strings.Contains(challenge.Manifest, "\t") {
			t.Fatalf("challenge %q manifest contains a YAML-invalid tab", challenge.ID)
		}
		for _, args := range challenge.Setup {
			if len(args) == 0 {
				t.Fatalf("challenge %q has an empty setup command", challenge.ID)
			}
		}
		for _, probe := range challenge.Probes {
			if len(probe.Args) == 0 || probe.Pending == "" {
				t.Fatalf("challenge %q has an incomplete probe: %#v", challenge.ID, probe)
			}
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
