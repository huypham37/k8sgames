package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/huypham37/k8sgames/internal/game"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type fileDescriptor interface {
	Fd() uintptr
}

type socketWriter struct {
	connection *websocket.Conn
	mu         sync.Mutex
}

type terminalMessage struct {
	Type    string `json:"type"`
	Columns uint16 `json:"columns,omitempty"`
	Rows    uint16 `json:"rows,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewClient(baseURL string, client *http.Client) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid server URL %q", baseURL)
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}, nil
}

func (c *Client) Challenges(ctx context.Context) ([]game.Challenge, error) {
	var challenges []game.Challenge
	if err := c.request(ctx, http.MethodGet, "/v1/challenges", "", nil, &challenges); err != nil {
		return nil, err
	}
	return challenges, nil
}

func (c *Client) Play(ctx context.Context, challenge string, input io.Reader, output io.Writer) (bool, error) {
	fmt.Fprintln(output, "Preparing your Kubernetes lab...")
	var session game.CreatedSession
	if err := c.request(ctx, http.MethodPost, "/v1/sessions", "", map[string]string{"challenge": challenge}, &session); err != nil {
		return false, err
	}
	defer c.request(context.Background(), http.MethodDelete, "/v1/sessions/"+session.ID, session.Token, nil, nil)

	fmt.Fprintf(output, "\nK8s Games — %s\n", session.Challenge.Title)
	fmt.Fprintf(output, "Objective: %s\n", session.Challenge.Objective)
	fmt.Fprintln(output, "Native shell ready. Use Tab completion, check, hint, objective, or exit.")
	return c.attach(ctx, session, input, output)
}

func (c *Client) attach(ctx context.Context, session game.CreatedSession, input io.Reader, output io.Writer) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	in, inputOK := input.(fileDescriptor)
	out, outputOK := output.(fileDescriptor)
	if !inputOK || !outputOK || !term.IsTerminal(int(in.Fd())) {
		cancel()
		return false, fmt.Errorf("an interactive terminal is required")
	}

	endpoint, err := c.terminalURL(session.ID)
	if err != nil {
		cancel()
		return false, err
	}
	header := http.Header{"Authorization": []string{"Bearer " + session.Token}}
	connection, response, err := websocket.DefaultDialer.DialContext(ctx, endpoint, header)
	if err != nil {
		cancel()
		if response != nil {
			return false, fmt.Errorf("terminal connection failed: %s", response.Status)
		}
		return false, err
	}
	writer := &socketWriter{connection: connection}

	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		cancel()
		_ = connection.Close()
		return false, err
	}

	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		if copyInput(ctx, input, int(in.Fd()), writer) != nil && ctx.Err() == nil {
			_ = connection.Close()
		}
	}()
	stopResize := watchResize(ctx, int(out.Fd()), writer)
	go func() {
		<-ctx.Done()
		_ = connection.Close()
	}()

	completed := false
	var result error
	for {
		kind, data, err := connection.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || ctx.Err() != nil {
				break
			}
			result = err
			break
		}
		if kind == websocket.BinaryMessage {
			if _, err := output.Write(data); err != nil {
				result = err
				break
			}
			continue
		}
		var message terminalMessage
		if json.Unmarshal(data, &message) != nil {
			continue
		}
		switch message.Type {
		case "complete":
			fmt.Fprintf(output, "\r\n\r\n✓ %s\r\n", message.Message)
			completed = true
		case "progress":
			fmt.Fprintf(output, "\r\n%s\r\n", message.Message)
		case "error":
			result = fmt.Errorf("%s", message.Message)
		}
		if completed || result != nil {
			break
		}
	}
	cancel()
	stopResize()
	_ = connection.Close()
	<-inputDone
	_ = term.Restore(int(in.Fd()), state)
	return completed, result
}

func (c *Client) terminalURL(id string) (string, error) {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	if endpoint.Scheme == "https" {
		endpoint.Scheme = "wss"
	} else {
		endpoint.Scheme = "ws"
	}
	endpoint.Path += "/v1/sessions/" + id + "/terminal"
	return endpoint.String(), nil
}

func (w *socketWriter) write(kind int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.connection.WriteMessage(kind, data)
}

func (w *socketWriter) event(message terminalMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return w.write(websocket.TextMessage, data)
}

func copyInput(ctx context.Context, input io.Reader, fd int, writer *socketWriter) error {
	buffer := make([]byte, 32<<10)
	for {
		if ctx.Err() != nil {
			return nil
		}
		ready, err := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, int((100 * time.Millisecond).Milliseconds()))
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if ready == 0 {
			continue
		}
		count, err := input.Read(buffer)
		if count > 0 {
			if writeErr := writer.write(websocket.BinaryMessage, buffer[:count]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			return err
		}
	}
}

func watchResize(ctx context.Context, fd int, writer *socketWriter) func() {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, syscall.SIGWINCH)
	send := func() {
		columns, rows, err := term.GetSize(fd)
		if err == nil {
			_ = writer.event(terminalMessage{Type: "resize", Columns: uint16(columns), Rows: uint16(rows)})
		}
	}
	send()
	go func() {
		for {
			select {
			case <-signals:
				send()
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

func (c *Client) request(ctx context.Context, method, path, token string, body, target any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		var problem map[string]string
		_ = json.NewDecoder(response.Body).Decode(&problem)
		if problem["error"] != "" {
			return fmt.Errorf("%s", problem["error"])
		}
		return fmt.Errorf("server returned %s", response.Status)
	}
	if target != nil && response.StatusCode != http.StatusNoContent {
		return json.NewDecoder(response.Body).Decode(target)
	}
	return nil
}
