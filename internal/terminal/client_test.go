package terminal

import (
	"net/http"
	"testing"
)

func TestTerminalURL(t *testing.T) {
	client, err := NewClient("https://games.example.com/api", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.terminalURL("session-1")
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://games.example.com/api/v1/sessions/session-1/terminal"
	if got != want {
		t.Fatalf("terminalURL() = %q, want %q", got, want)
	}
}
