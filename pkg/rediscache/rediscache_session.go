package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ── Session management ─────────────────────────
// Stores complete session data including event, questions, summary, transcription.
//   Key → session:<sessionID>

func (c *RedisCacheClient) SaveSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	if session.SessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Update timestamp
	now := time.Now().UnixMilli()
	if session.CreatedAt == 0 {
		session.CreatedAt = now
	}
	session.UpdatedAt = now

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session %s: %w", session.SessionID, err)
	}

	key := sessionKeyPrefix + session.SessionID
	if err := c.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("save session %s: %w", session.SessionID, err)
	}
	return nil
}

func (c *RedisCacheClient) FindSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	key := sessionKeyPrefix + sessionID
	raw, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("find session %s: %w", sessionID, err)
	}

	var session Session
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, fmt.Errorf("unmarshal session %s: %w", sessionID, err)
	}

	return &session, nil
}

func (c *RedisCacheClient) Close() error {
	return c.client.Close()
}
