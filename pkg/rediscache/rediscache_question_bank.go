package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"tal_assistant/pkg/adkutils"
)

// ── Question bank ──────────────────────────────
// Storage layout: one Redis hash per interview.
//   Key   → qbank:<interviewID>
//   Field → <questionID>   (e.g. "TLQ001")
//   Value → JSON-encoded QuestionBankQuestion
//
// We write each field individually through a pipeline so we never hit
// variadic-argument ambiguity in go-redis regardless of patch version.

func (c *RedisCacheClient) SaveQuestionBank(
	ctx context.Context,
	interviewID string,
	questions []adkutils.QuestionBankQuestion,
) error {
	if len(questions) == 0 {
		return nil
	}
	key := questionBankKeyPrefix + interviewID

	pipe := c.client.Pipeline()
	for i, q := range questions {
		// Stamp the 1-based position so order is persisted in Redis.
		// Only set it when it hasn't been assigned yet so that an explicit
		// order from the generator is never overwritten.
		if q.Order == 0 {
			q.Order = i + 1
		}
		data, err := json.Marshal(q)
		if err != nil {
			return fmt.Errorf("marshal question %s: %w", q.ID, err)
		}
		// One HSET key field value per question — no variadic ambiguity.
		pipe.HSet(ctx, key, q.ID, string(data))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("save question bank for interview %s: %w", interviewID, err)
	}
	return nil
}

// FindQuestionBank returns every question in the bank as a sorted array ordered by the Order field.
func (c *RedisCacheClient) FindQuestionBank(
	ctx context.Context,
	interviewID string,
) ([]adkutils.QuestionBankQuestion, error) {
	key := questionBankKeyPrefix + interviewID
	raw, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("find question bank for interview %s: %w", interviewID, err)
	}
	result := make([]adkutils.QuestionBankQuestion, 0, len(raw))
	for questionID, data := range raw {
		var q adkutils.QuestionBankQuestion
		if err := json.Unmarshal([]byte(data), &q); err != nil {
			return nil, fmt.Errorf("unmarshal question %s: %w", questionID, err)
		}
		result = append(result, q)
	}
	// Sort by Order field (0 values go last), then by ID as tiebreaker
	sort.Slice(result, func(i, j int) bool {
		oi, oj := result[i].Order, result[j].Order
		if oi != oj {
			if oi == 0 {
				return false
			}
			if oj == 0 {
				return true
			}
			return oi < oj
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// FindQuestionByID fetches a single question directly by its ID using HGET —
// more efficient than loading the whole bank when only one question is needed.
func (c *RedisCacheClient) FindQuestionByID(
	ctx context.Context,
	interviewID string,
	questionID string,
) (*adkutils.QuestionBankQuestion, error) {
	key := questionBankKeyPrefix + interviewID
	data, err := c.client.HGet(ctx, key, questionID).Result()
	if err != nil {
		return nil, fmt.Errorf("find question %s in bank for interview %s: %w", questionID, interviewID, err)
	}
	var q adkutils.QuestionBankQuestion
	if err := json.Unmarshal([]byte(data), &q); err != nil {
		return nil, fmt.Errorf("unmarshal question %s: %w", questionID, err)
	}
	return &q, nil
}

// ── Current question pointer ───────────────────

func (c *RedisCacheClient) UpsertCurrentQuestionPointer(
	ctx context.Context,
	interviewID string,
	questionID string,
) error {
	key := currentQuestionKeyPrefix + interviewID
	if err := c.client.Set(ctx, key, questionID, 0).Err(); err != nil {
		return fmt.Errorf("upsert current question pointer for interview %s: %w", interviewID, err)
	}
	return nil
}
