package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cletus/internal/api"
)

// Session represents a saved conversation session
type Session struct {
	ID        string           `json:"id"`
	Model     string           `json:"model"`
	Messages  []api.APIMessage `json:"messages"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Metadata  SessionMetadata  `json:"metadata"`
}

// SessionMetadata holds session metadata
type SessionMetadata struct {
	Title     string `json:"title"`
	ToolCalls int    `json:"tool_calls"`
}

// Store manages session persistence
type Store struct {
	sessionDir string
}

// NewStore creates a new session store
func NewStore(sessionDir string) *Store {
	if sessionDir == "" {
		home, _ := os.UserHomeDir()
		sessionDir = filepath.Join(home, ".config", "cletus", "sessions")
	}
	return &Store{
		sessionDir: sessionDir,
	}
}

// EnsureDir ensures the session directory exists
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.sessionDir, 0755)
}

// Create creates a new session
func (s *Store) Create(model string) (*Session, error) {
	if err := s.EnsureDir(); err != nil {
		return nil, err
	}

	now := time.Now()
	session := &Session{
		ID:        fmt.Sprintf("session-%d", now.Unix()),
		Model:     model,
		Messages:  make([]api.APIMessage, 0),
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: SessionMetadata{
			Title: "New Session",
		},
	}

	return session, nil
}

// Save saves a session to disk
func (s *Store) Save(session *Session) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	session.UpdatedAt = time.Now()

	path := filepath.Join(s.sessionDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Load loads a session from disk
func (s *Store) Load(id string) (*Session, error) {
	path := filepath.Join(s.sessionDir, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return &session, nil
}

// List lists all sessions
func (s *Store) List() ([]Session, error) {
	entries, err := os.ReadDir(s.sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json
		session, err := s.Load(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, *session)
	}

	// Sort by updated time, newest first
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].UpdatedAt.After(sessions[i].UpdatedAt) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	return sessions, nil
}

// Delete deletes a session
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.sessionDir, id+".json")
	return os.Remove(path)
}

// GetLatest returns the most recent session
func (s *Store) GetLatest() (*Session, error) {
	sessions, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	return &sessions[0], nil
}
