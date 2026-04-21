package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
)

// sessionAgentResponseKey returns the Redis list key that holds all agent
// responses recorded during a session. Responses are appended in chronological
// order so the full agent conversation can be replayed by reading the list
// from head to tail.
//
//	Format: session:<sessionID>:agent_responses
//	Example: session:ses_abc123:agent_responses
func sessionAgentResponseKey(sessionID string) string {
	return fmt.Sprintf("session:%s:agent_responses", sessionID)
}

// AgentResponseCreate appends an agent response to the session's response log.
// Responses are stored as JSON-encoded values in a Redis list (RPUSH) so they
// can be retrieved in the exact order they were produced across all agents.
//
// The Agent field on the response identifies which agent produced the output,
// allowing callers to filter by agent after fetching the full list if needed.
func (c *RedisCacheClient) AgentResponseCreate(ctx context.Context, sessionID string, response *AgentResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal agent response for session %s: %w", sessionID, err)
	}

	if err := c.client.RPush(ctx, sessionAgentResponseKey(sessionID), string(data)).Err(); err != nil {
		return fmt.Errorf("append agent response for session %s: %w", sessionID, err)
	}

	return nil
}
