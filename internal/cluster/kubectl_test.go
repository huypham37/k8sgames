package cluster

import "testing"

func TestValidNamespace(t *testing.T) {
	if !validNamespace("k8sgames-a1b2c3") {
		t.Fatal("expected generated namespace to be valid")
	}
	for _, namespace := range []string{"default", "k8sgames-UPPER", "k8sgames-bad_name"} {
		if validNamespace(namespace) {
			t.Fatalf("expected %q to be invalid", namespace)
		}
	}
}
