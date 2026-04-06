/*
Agent Runtime — Fantasy (Go)

Session management for the daemon server.
Tracks conversation sessions with message history so the console
can maintain multiple concurrent chat threads against a single agent.
*/
package main

import (
	"sort"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/google/uuid"
)

// Session represents a single conversation with the agent.
type Session struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Messages     []fantasy.Message `json:"messages"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	MessageCount int               `json:"message_count"`
}

// SessionInfo is the API-facing representation of a session (times as RFC3339).
type SessionInfo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

// Info returns an API-safe representation with RFC3339 timestamps.
func (s *Session) Info() SessionInfo {
	return SessionInfo{
		ID:           s.ID,
		Title:        s.Title,
		CreatedAt:    s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    s.UpdatedAt.Format(time.RFC3339),
		MessageCount: s.MessageCount,
	}
}

// SessionStore is an in-memory store for agent sessions.
type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionStore creates an empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

// Create makes a new session. If title is empty, "New Session" is used.
func (ss *SessionStore) Create(title string) *Session {
	if title == "" {
		title = "New Session"
	}
	now := time.Now()
	s := &Session{
		ID:        uuid.NewString(),
		Title:     title,
		Messages:  []fantasy.Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	ss.mu.Lock()
	ss.sessions[s.ID] = s
	ss.mu.Unlock()

	return s
}

// Get retrieves a session by ID.
func (ss *SessionStore) Get(id string) (*Session, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	s, ok := ss.sessions[id]
	return s, ok
}

// List returns all sessions sorted by UpdatedAt descending (most recent first).
func (ss *SessionStore) List() []*Session {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	list := make([]*Session, 0, len(ss.sessions))
	for _, s := range ss.sessions {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

// Delete removes a session by ID. Returns true if the session existed.
func (ss *SessionStore) Delete(id string) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	_, ok := ss.sessions[id]
	if ok {
		delete(ss.sessions, id)
	}
	return ok
}

// AppendMessages adds messages to a session and updates the timestamp and count.
func (ss *SessionStore) AppendMessages(id string, msgs ...fantasy.Message) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	s, ok := ss.sessions[id]
	if !ok {
		return
	}
	s.Messages = append(s.Messages, msgs...)
	s.MessageCount += len(msgs)
	s.UpdatedAt = time.Now()
}

// GetMessages returns the message history for a session.
func (ss *SessionStore) GetMessages(id string) []fantasy.Message {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	s, ok := ss.sessions[id]
	if !ok {
		return nil
	}
	// Return a copy to avoid races.
	out := make([]fantasy.Message, len(s.Messages))
	copy(out, s.Messages)
	return out
}

// UpdateTitle changes the title of a session.
func (ss *SessionStore) UpdateTitle(id string, title string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	s, ok := ss.sessions[id]
	if !ok {
		return
	}
	s.Title = title
	s.UpdatedAt = time.Now()
}
