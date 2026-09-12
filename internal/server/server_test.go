package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/huypham37/k8sgames/internal/game"
)

type testCluster struct {
	fixed    bool
	terminal *testTerminal
}

func (c *testCluster) Provision(context.Context, string, game.Challenge) error { return nil }
func (c *testCluster) Inspect(_ context.Context, _ string, args []string) game.Result {
	if !c.fixed {
		return game.Result{Output: "nginx:image-does-not-exist"}
	}
	if strings.Contains(strings.Join(args, " "), "availableReplicas") {
		return game.Result{Output: "2"}
	}
	return game.Result{Output: "nginx:1.27-alpine"}
}
func (c *testCluster) Delete(context.Context, string) error { return nil }
func (c *testCluster) Ping(context.Context) error           { return nil }
func (c *testCluster) Cleanup(context.Context) error        { return nil }
func (c *testCluster) OpenTerminal(context.Context, string, game.Challenge, game.TerminalSize) (game.Terminal, error) {
	if c.terminal == nil {
		c.terminal = newTestTerminal()
	}
	return c.terminal, nil
}

type testTerminal struct {
	output  chan []byte
	resized chan game.TerminalSize
	done    chan struct{}
	once    sync.Once
}

func newTestTerminal() *testTerminal {
	return &testTerminal{output: make(chan []byte, 1), resized: make(chan game.TerminalSize, 1), done: make(chan struct{})}
}

func (t *testTerminal) Read(buffer []byte) (int, error) {
	select {
	case data := <-t.output:
		return copy(buffer, data), nil
	case <-t.done:
		return 0, io.EOF
	}
}

func (t *testTerminal) Write(data []byte) (int, error) {
	cloned := append([]byte(nil), data...)
	select {
	case t.output <- cloned:
		return len(data), nil
	case <-t.done:
		return 0, io.EOF
	}
}

func (t *testTerminal) Close() error {
	t.once.Do(func() { close(t.done) })
	return nil
}

func (t *testTerminal) Resize(size game.TerminalSize) error {
	t.resized <- size
	return nil
}

func TestSessionAPI(t *testing.T) {
	manager := game.NewManager(&testCluster{}, game.NewCatalog(), time.Minute, 2)
	api := New(manager)

	response := request(t, api, http.MethodPost, "/v1/sessions", "", `{"challenge":"broken-image"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body.String())
	}
	var created game.CreatedSession
	decodeResponse(t, response, &created)

	unauthorized := request(t, api, http.MethodGet, "/v1/sessions/"+created.ID, "wrong", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	terminal := request(t, api, http.MethodGet, "/v1/sessions/"+created.ID+"/terminal", "wrong", "")
	if terminal.Code != http.StatusUnauthorized {
		t.Fatalf("terminal status = %d", terminal.Code)
	}

	deleted := request(t, api, http.MethodDelete, "/v1/sessions/"+created.ID, created.Token, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleted.Code)
	}
}

func TestTerminalWebSocket(t *testing.T) {
	remote := newTestTerminal()
	provider := &testCluster{terminal: remote}
	manager := game.NewManager(provider, game.NewCatalog(), time.Minute, 1)
	created, err := manager.Create(context.Background(), "broken-image")
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(New(manager))
	defer api.Close()

	endpoint := "ws" + strings.TrimPrefix(api.URL, "http") + "/v1/sessions/" + created.ID + "/terminal"
	header := http.Header{"Authorization": []string{"Bearer " + created.Token}}
	connection, response, err := websocket.DefaultDialer.Dial(endpoint, header)
	if err != nil {
		if response != nil {
			t.Fatalf("dial: %v (%s)", err, response.Status)
		}
		t.Fatal(err)
	}
	defer connection.Close()

	if err := connection.WriteMessage(websocket.BinaryMessage, []byte("kubectl get pods\r")); err != nil {
		t.Fatal(err)
	}
	kind, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if kind != websocket.BinaryMessage || string(data) != "kubectl get pods\r" {
		t.Fatalf("terminal response = %q (type %d)", data, kind)
	}

	resize := `{"type":"resize","columns":132,"rows":38}`
	if err := connection.WriteMessage(websocket.TextMessage, []byte(resize)); err != nil {
		t.Fatal(err)
	}
	select {
	case size := <-remote.resized:
		if size.Columns != 132 || size.Rows != 38 {
			t.Fatalf("resize = %#v", size)
		}
	case <-time.After(time.Second):
		t.Fatal("resize was not forwarded")
	}
}

func TestTerminalCheckCommandReportsProgress(t *testing.T) {
	remote := newTestTerminal()
	provider := &testCluster{terminal: remote}
	manager := game.NewManager(provider, game.NewCatalog(), time.Minute, 1)
	created, err := manager.Create(context.Background(), "broken-image")
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(New(manager))
	defer api.Close()

	endpoint := "ws" + strings.TrimPrefix(api.URL, "http") + "/v1/sessions/" + created.ID + "/terminal"
	header := http.Header{"Authorization": []string{"Bearer " + created.Token}}
	connection, _, err := websocket.DefaultDialer.Dial(endpoint, header)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	go func() {
		remote.output <- checkSequence[:5]
		remote.output <- checkSequence[5:]
	}()
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	kind, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var message terminalMessage
	if kind != websocket.TextMessage || json.Unmarshal(data, &message) != nil {
		t.Fatalf("check response = %q (type %d)", data, kind)
	}
	if message.Type != "progress" || message.Message == "" {
		t.Fatalf("check response = %#v", message)
	}
}

func request(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
