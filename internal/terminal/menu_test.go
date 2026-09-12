package terminal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/huypham37/k8sgames/internal/game"
)

func TestMenuSelectsChallenge(t *testing.T) {
	var output bytes.Buffer
	menu := NewMenu(strings.NewReader("2\n"), &output)
	selected, err := menu.Select(game.NewCatalog().All(), func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0] != "scale-up" {
		t.Fatalf("selected = %v", selected)
	}
	if !strings.Contains(output.String(), "Foundations\n") || !strings.Contains(output.String(), "Progress: 0/25") {
		t.Fatalf("menu output = %q", output.String())
	}
}

func TestMenuSelectsRemaining(t *testing.T) {
	menu := NewMenu(strings.NewReader("a\n"), &bytes.Buffer{})
	selected, err := menu.Select(game.NewCatalog().All(), func(id string) bool { return id == "broken-image" })
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 24 || selected[0] != "first-pod" {
		t.Fatalf("selected = %v", selected)
	}
}
