package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/huypham37/k8sgames/internal/game"
)

var terminalUpgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin:      func(*http.Request) bool { return true },
}

type terminalMessage struct {
	Type    string `json:"type"`
	Columns uint16 `json:"columns,omitempty"`
	Rows    uint16 `json:"rows,omitempty"`
	Message string `json:"message,omitempty"`
}

type socketWriter struct {
	connection *websocket.Conn
	mu         sync.Mutex
}

var checkSequence = []byte("\x1eK8SGAMES_CHECK\x1f")

type terminalOutput struct {
	writer  *socketWriter
	checks  chan<- struct{}
	pending []byte
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

func (s *Server) terminal(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	token := bearerToken(request)
	if _, err := s.manager.Get(id, token); err != nil {
		writeSessionError(response, err)
		return
	}

	connection, err := terminalUpgrader.Upgrade(response, request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(64 << 10)

	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	remote, err := s.manager.OpenTerminal(ctx, id, token, game.TerminalSize{Columns: 120, Rows: 40})
	writer := &socketWriter{connection: connection}
	if err != nil {
		_ = writer.event(terminalMessage{Type: "error", Message: err.Error()})
		return
	}
	defer remote.Close()

	errors := make(chan error, 2)
	checks := make(chan struct{}, 1)
	go copyTerminal(remote, writer, checks, errors)
	go copyClient(connection, remote, errors)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-errors:
			return
		case <-checks:
			if s.check(ctx, id, token, writer, true) {
				return
			}
		case <-ticker.C:
			if s.check(ctx, id, token, writer, false) {
				return
			}
		}
	}
}

func (s *Server) check(ctx context.Context, id, token string, writer *socketWriter, reportProgress bool) bool {
	session, err := s.manager.Check(ctx, id, token)
	if err != nil {
		_ = writer.event(terminalMessage{Type: "error", Message: err.Error()})
		return true
	}
	if session.Completed {
		_ = writer.event(terminalMessage{Type: "complete", Message: "Challenge complete!"})
		return true
	}
	if reportProgress {
		_ = writer.event(terminalMessage{Type: "progress", Message: session.Progress})
	}
	return false
}

func copyTerminal(remote game.Terminal, writer *socketWriter, checks chan<- struct{}, errors chan<- error) {
	output := &terminalOutput{writer: writer, checks: checks}
	buffer := make([]byte, 32<<10)
	for {
		count, err := remote.Read(buffer)
		if count > 0 {
			if writeErr := output.write(buffer[:count]); writeErr != nil {
				errors <- writeErr
				return
			}
		}
		if err != nil {
			_ = output.flush()
			errors <- err
			return
		}
	}
}

func (o *terminalOutput) write(data []byte) error {
	data = append(append([]byte(nil), o.pending...), data...)
	o.pending = o.pending[:0]
	for {
		index := bytes.Index(data, checkSequence)
		if index < 0 {
			keep := markerPrefix(data)
			o.pending = append(o.pending, data[len(data)-keep:]...)
			if len(data) > keep {
				return o.writer.write(websocket.BinaryMessage, data[:len(data)-keep])
			}
			return nil
		}
		if index > 0 {
			if err := o.writer.write(websocket.BinaryMessage, data[:index]); err != nil {
				return err
			}
		}
		select {
		case o.checks <- struct{}{}:
		default:
		}
		data = data[index+len(checkSequence):]
	}
}

func (o *terminalOutput) flush() error {
	if len(o.pending) == 0 {
		return nil
	}
	err := o.writer.write(websocket.BinaryMessage, o.pending)
	o.pending = nil
	return err
}

func markerPrefix(data []byte) int {
	limit := min(len(data), len(checkSequence)-1)
	for size := limit; size > 0; size-- {
		if bytes.Equal(data[len(data)-size:], checkSequence[:size]) {
			return size
		}
	}
	return 0
}

func copyClient(connection *websocket.Conn, remote game.Terminal, failures chan<- error) {
	for {
		kind, data, err := connection.ReadMessage()
		if err != nil {
			failures <- err
			return
		}
		switch kind {
		case websocket.BinaryMessage:
			if _, err := remote.Write(data); err != nil {
				failures <- err
				return
			}
		case websocket.TextMessage:
			var message terminalMessage
			if err := json.Unmarshal(data, &message); err != nil {
				continue
			}
			if message.Type == "resize" && message.Columns > 0 && message.Rows > 0 {
				if err := remote.Resize(game.TerminalSize{Columns: message.Columns, Rows: message.Rows}); err != nil && !errors.Is(err, io.EOF) {
					failures <- err
					return
				}
			}
		}
	}
}
