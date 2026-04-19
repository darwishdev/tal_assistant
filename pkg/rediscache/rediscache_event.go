package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"tal_assistant/pkg/workableclient"
)

// ── Event data ─────────────────────────────────
// Stores the Workable EventFindResult so it can be re-used
// without an additional API call.
//   Key → event:<eventID>

func (c *RedisCacheClient) SaveEventData(ctx context.Context, eventID string, event *workableclient.EventFindResult) error {
	if event == nil {
		return fmt.Errorf("event data cannot be nil")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event data for %s: %w", eventID, err)
	}

	key := eventDataKeyPrefix + eventID
	if err := c.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("save event data for %s: %w", eventID, err)
	}
	return nil
}

func (c *RedisCacheClient) FindEventData(ctx context.Context, eventID string) (*workableclient.EventFindResult, error) {
	key := eventDataKeyPrefix + eventID
	raw, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("find event data for %s: %w", eventID, err)
	}

	var event workableclient.EventFindResult
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return nil, fmt.Errorf("unmarshal event data for %s: %w", eventID, err)
	}

	return &event, nil
}
