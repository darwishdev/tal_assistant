package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *RedisCacheClient) EventCreate(ctx context.Context, eventID string, event *Event) error {
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

func (c *RedisCacheClient) EventFind(ctx context.Context, eventID string) (*Event, error) {
	key := eventDataKeyPrefix + eventID
	raw, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("find event data for %s: %w", eventID, err)
	}

	var event Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return nil, fmt.Errorf("unmarshal event data for %s: %w", eventID, err)
	}

	return &event, nil
}
