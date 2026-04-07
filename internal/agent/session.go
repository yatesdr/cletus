package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cletus/internal/api"
)

// Session represents a conversation session
type Session struct {
	ID        string           `json:"id"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Model     string           `json:"model"`
	Messages  []api.APIMessage `json:"messages"`
	Metadata  SessionMetadata  `json:"metadata"`
}

// SessionMetadata holds session metadata
type SessionMetadata struct {
	Title         string `json:"title"`
	FirstUserMsg  string `json:"first_user_msg"`
	LastUserMsg   string `json:"last_user_msg"`
	MessageCount  int    `json:"message_count"`
	TokenCount    int    `json:"token_count"`
}

// SessionStore manages session persistence
type SessionStore struct {
	sessionsDir string
}

// NewSessionStore creates a new session store
func NewSessionStore(sessionsDir string) *SessionStore {
	return &SessionStore{
		sessionsDir: sessionsDir,
	}
}

// EnsureDir ensures the sessions directory exists
func (s *SessionStore) EnsureDir() error {
	return os.MkdirAll(s.sessionsDir, 0755)
}

// Create creates a new session
func (s *SessionStore) Create(model string) (*Session, error) {
	if err := s.EnsureDir(); err != nil {
		return nil, err
	}

	session := &Session{
		ID:        generateSessionID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Model:     model,
		Messages:  make([]api.APIMessage, 0),
		Metadata: SessionMetadata{
			MessageCount: 0,
		},
	}

	if err := s.Save(session); err != nil {
		return nil, err
	}

	return session, nil
}

// Save saves a session
func (s *SessionStore) Save(session *Session) error {
	session.UpdatedAt = time.Now()
	
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(s.sessionsDir, session.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// Load loads a session by ID
func (s *SessionStore) Load(sessionID string) (*Session, error) {
	path := filepath.Join(s.sessionsDir, sessionID+".json")
	
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}

	return &session, nil
}

// Delete deletes a session by ID
func (s *SessionStore) Delete(sessionID string) error {
	path := filepath.Join(s.sessionsDir, sessionID+".json")
	return os.Remove(path)
}

// List lists all sessions, newest first
func (s *SessionStore) List() ([]*Session, error) {
	if err := s.EnsureDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.sessionsDir)
	if err != nil {
		return nil, err
	}

	sessions := make([]*Session, 0, len(entries))
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		
		sessionID := entry.Name()[:len(entry.Name())-5]
		session, err := s.Load(sessionID)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
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

// GetRecent returns the most recent session
func (s *SessionStore) GetRecent() (*Session, error) {
	sessions, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}
	return sessions[0], nil
}

// getFirstUserContent extracts the first user message content from a message
func getFirstUserContent(msg api.APIMessage) string {
	for _, block := range msg.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

// UpdateMessageCount updates the session message count
func (s *SessionStore) UpdateMessageCount(session *Session) {
	session.Metadata.MessageCount = len(session.Messages)
	
	// Update title from first message - Content is now []ContentBlock
	if len(session.Messages) > 0 && session.Metadata.Title == "" {
		for _, msg := range session.Messages {
			if msg.Role == "user" {
				content := getFirstUserContent(msg)
				if content != "" {
					session.Metadata.FirstUserMsg = content
					title := content
					if len(title) > 50 {
						title = title[:50] + "..."
					}
					session.Metadata.Title = title
				}
				break
			}
		}
	}
	
	// Update last message
	for i := len(session.Messages) - 1; i >= 0; i-- {
		if session.Messages[i].Role == "user" {
			content := getFirstUserContent(session.Messages[i])
			if content != "" {
				session.Metadata.LastUserMsg = content
			}
			break
		}
	}
}

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
