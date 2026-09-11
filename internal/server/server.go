package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/huypham37/k8sgames/internal/game"
)

type Server struct {
	manager *game.Manager
	mux     *http.ServeMux
}

func New(manager *game.Manager) *Server {
	server := &Server{manager: manager, mux: http.NewServeMux()}
	server.mux.HandleFunc("GET /healthz", server.health)
	server.mux.HandleFunc("GET /v1/challenges", server.challenges)
	server.mux.HandleFunc("POST /v1/sessions", server.createSession)
	server.mux.HandleFunc("GET /v1/sessions/{id}", server.getSession)
	server.mux.HandleFunc("GET /v1/sessions/{id}/terminal", server.terminal)
	server.mux.HandleFunc("DELETE /v1/sessions/{id}", server.deleteSession)
	return server
}

func (s *Server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	s.mux.ServeHTTP(response, request)
}

func (s *Server) health(response http.ResponseWriter, request *http.Request) {
	if err := s.manager.Ping(request.Context()); err != nil {
		writeError(response, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) challenges(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, s.manager.Challenges())
}

func (s *Server) createSession(response http.ResponseWriter, request *http.Request) {
	var body struct {
		Challenge string `json:"challenge"`
	}
	if err := decode(request, &body); err != nil {
		writeError(response, http.StatusBadRequest, err)
		return
	}
	if body.Challenge == "" {
		body.Challenge = "broken-image"
	}
	session, err := s.manager.Create(request.Context(), body.Challenge)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, game.ErrFull) {
			status = http.StatusServiceUnavailable
		} else if strings.Contains(err.Error(), "unknown challenge") {
			status = http.StatusBadRequest
		}
		writeError(response, status, err)
		return
	}
	writeJSON(response, http.StatusCreated, session)
}

func (s *Server) getSession(response http.ResponseWriter, request *http.Request) {
	session, err := s.manager.Get(request.PathValue("id"), bearerToken(request))
	if err != nil {
		writeSessionError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, session)
}

func (s *Server) deleteSession(response http.ResponseWriter, request *http.Request) {
	err := s.manager.Delete(request.Context(), request.PathValue("id"), bearerToken(request))
	if err != nil {
		writeSessionError(response, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func decode(request *http.Request, value any) error {
	request.Body = http.MaxBytesReader(nil, request.Body, 64<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func bearerToken(request *http.Request) string {
	value := request.Header.Get("Authorization")
	return strings.TrimPrefix(value, "Bearer ")
}

func writeSessionError(response http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, game.ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, game.ErrUnauthorized) {
		status = http.StatusUnauthorized
	}
	writeError(response, status, err)
}

func writeError(response http.ResponseWriter, status int, err error) {
	writeJSON(response, status, map[string]string{"error": err.Error()})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
