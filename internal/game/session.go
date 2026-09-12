package game

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("session not found")
	ErrUnauthorized = errors.New("invalid session token")
	ErrFull         = errors.New("server has reached its session limit")
)

type Session struct {
	ID        string    `json:"id"`
	Namespace string    `json:"namespace"`
	Challenge Challenge `json:"challenge"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Completed bool      `json:"completed"`
	Progress  string    `json:"progress"`
	token     string
}

type CreatedSession struct {
	Session
	Token string `json:"token"`
}

type Manager struct {
	cluster  Cluster
	catalog  *Catalog
	ttl      time.Duration
	max      int
	mu       sync.RWMutex
	pending  int
	sessions map[string]*Session
}

func NewManager(cluster Cluster, catalog *Catalog, ttl time.Duration, max int) *Manager {
	return &Manager{cluster: cluster, catalog: catalog, ttl: ttl, max: max, sessions: map[string]*Session{}}
}

func (m *Manager) Create(ctx context.Context, challengeID string) (CreatedSession, error) {
	m.mu.Lock()
	if len(m.sessions)+m.pending >= m.max {
		m.mu.Unlock()
		return CreatedSession{}, ErrFull
	}
	m.pending++
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.pending--
		m.mu.Unlock()
	}()

	challenge, err := m.catalog.Find(challengeID)
	if err != nil {
		return CreatedSession{}, err
	}
	id, err := randomID(6)
	if err != nil {
		return CreatedSession{}, err
	}
	token, err := randomID(24)
	if err != nil {
		return CreatedSession{}, err
	}

	now := time.Now().UTC()
	session := &Session{
		ID: id, Namespace: "k8sgames-" + id, Challenge: challenge,
		CreatedAt: now, ExpiresAt: now.Add(m.ttl), Progress: challenge.Objective, token: token,
	}
	if err := m.cluster.Provision(ctx, session.Namespace, challenge); err != nil {
		return CreatedSession{}, fmt.Errorf("provision session: %w", err)
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()
	return CreatedSession{Session: *session, Token: token}, nil
}

func (m *Manager) OpenTerminal(ctx context.Context, id, token string, size TerminalSize) (Terminal, error) {
	session, err := m.authorize(id, token)
	if err != nil {
		return nil, err
	}
	return m.cluster.OpenTerminal(ctx, session.Namespace, session.Challenge, size)
}

func (m *Manager) Check(ctx context.Context, id, token string) (Session, error) {
	session, err := m.authorize(id, token)
	if err != nil {
		return Session{}, err
	}
	if session.Completed {
		return session, nil
	}
	completed, progress := m.grade(ctx, &session)
	m.mu.Lock()
	if current := m.sessions[id]; current != nil {
		current.Completed = completed
		current.Progress = progress
	}
	m.mu.Unlock()
	session.Completed = completed
	session.Progress = progress
	return session, nil
}

func (m *Manager) Get(id, token string) (Session, error) {
	session, err := m.authorize(id, token)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (m *Manager) Delete(ctx context.Context, id, token string) error {
	session, err := m.authorize(id, token)
	if err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	return m.cluster.Delete(ctx, session.Namespace)
}

func (m *Manager) DeleteExpired(ctx context.Context) {
	now := time.Now()
	m.mu.Lock()
	expired := make([]*Session, 0)
	for id, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			expired = append(expired, session)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()
	for _, session := range expired {
		_ = m.cluster.Delete(ctx, session.Namespace)
	}
}

func (m *Manager) Challenges() []Challenge { return m.catalog.All() }

func (m *Manager) Cleanup(ctx context.Context) error { return m.cluster.Cleanup(ctx) }

func (m *Manager) Ping(ctx context.Context) error { return m.cluster.Ping(ctx) }

func (m *Manager) authorize(id, token string) (Session, error) {
	m.mu.RLock()
	session := m.sessions[id]
	if session == nil {
		m.mu.RUnlock()
		return Session{}, ErrNotFound
	}
	snapshot := *session
	m.mu.RUnlock()
	if snapshot.token != token {
		return Session{}, ErrUnauthorized
	}
	if time.Now().After(snapshot.ExpiresAt) {
		return Session{}, ErrNotFound
	}
	return snapshot, nil
}

func (m *Manager) grade(ctx context.Context, session *Session) (bool, string) {
	for _, probe := range session.Challenge.Probes {
		result := m.cluster.Inspect(ctx, session.Namespace, probe.Args)
		output := strings.TrimSpace(result.Output)
		failed := result.ExitCode != 0
		switch {
		case probe.NotEmpty:
			failed = failed || output == ""
		case len(probe.Unordered) > 0:
			values := strings.Fields(output)
			slices.Sort(values)
			expected := append([]string(nil), probe.Unordered...)
			slices.Sort(expected)
			failed = failed || !slices.Equal(values, expected)
		case probe.AtLeast > 0:
			value, err := strconv.Atoi(output)
			failed = failed || err != nil || value < probe.AtLeast
		default:
			failed = failed || output != probe.Equals
		}
		if failed {
			return false, probe.Pending
		}
	}
	return true, "Challenge complete."
}

func randomID(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
