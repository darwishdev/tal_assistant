package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"tal_assistant/pkg/adkutils"

	"github.com/redis/go-redis/v9"
)

const (
	channelSignalDetected        = "signal_detected"
	channelSignalMapped          = "signal_mapped"
	channelNextQuestionIndicated = "next_question_indicated"
	channelNextQuestionExtended  = "next_question_extended"
)

// SignalDetectedEvent is published when the signaling agent emits a Q:/A:/UNCLEAR signal.
// The publisher must include the mapper session ID so the subscriber can route the call.
type SignalDetectedEvent struct {
	InterviewID     string `json:"interview_id"`
	UserID          string `json:"user_id"`
	TranscriptLine  string `json:"transcript_line"`
	Signal          string `json:"signal"` // Q:..., A:..., or UNCLEAR
	QAndA           string `json:"q_and_a"`
	SignalingSessionID string `json:"signaling_session_id"`
	MapperSessionID    string `json:"mapper_session_id"`
	IndicatorSessionID string `json:"indicator_session_id"`
	ExtenderSessionID  string `json:"extender_session_id"`
}

// SignalMappedEvent is published after the mapper resolves a signal to a question ID.
type SignalMappedEvent struct {
	InterviewID        string `json:"interview_id"`
	UserID             string `json:"user_id"`
	Signal             string `json:"signal"`
	QuestionID         string `json:"question_id"`
	QAndA              string `json:"q_and_a"`
	SignalingSessionID string `json:"signaling_session_id"`
	MapperSessionID    string `json:"mapper_session_id"`
	IndicatorSessionID string `json:"indicator_session_id"`
	ExtenderSessionID  string `json:"extender_session_id"`
}

// NextQuestionIndicatedEvent is published after the NQI agent evaluates the Q&A.
type NextQuestionIndicatedEvent struct {
	InterviewID        string `json:"interview_id"`
	UserID             string `json:"user_id"`
	Indication         string `json:"indication"` // F:..., C:..., or None
	CurrentQuestionID  string `json:"current_question_id"`
	SignalingSessionID string `json:"signaling_session_id"`
	MapperSessionID    string `json:"mapper_session_id"`
	IndicatorSessionID string `json:"indicator_session_id"`
	ExtenderSessionID  string `json:"extender_session_id"`
}

// NextQuestionExtendedEvent is published after the NQE agent builds the full question struct.
// Type is "followup" or "change"; ParentQuestionID is set only for follow-ups.
type NextQuestionExtendedEvent struct {
	InterviewID        string                        `json:"interview_id"`
	UserID             string                        `json:"user_id"`
	Question           adkutils.QuestionBankQuestion `json:"question"`
	Type               string                        `json:"type"`
	ParentQuestionID   string                        `json:"parent_question_id,omitempty"`
	SignalingSessionID string                        `json:"signaling_session_id"`
	MapperSessionID    string                        `json:"mapper_session_id"`
	IndicatorSessionID string                        `json:"indicator_session_id"`
}

type PublisherInterface interface {
	PublishSignalDetected(ctx context.Context, event SignalDetectedEvent) error
	PublishSignalMapped(ctx context.Context, event SignalMappedEvent) error
	PublishNextQuestionIndicated(ctx context.Context, event NextQuestionIndicatedEvent) error
	PublishNextQuestionExtended(ctx context.Context, event NextQuestionExtendedEvent) error
}

type RedisPublisher struct {
	client *redis.Client
}

func NewRedisPublisher() *RedisPublisher {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	return &RedisPublisher{
		client: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

func publish(ctx context.Context, client *redis.Client, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload for channel %s: %w", channel, err)
	}
	if err := client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("publish to channel %s: %w", channel, err)
	}
	return nil
}

func (p *RedisPublisher) PublishSignalDetected(ctx context.Context, event SignalDetectedEvent) error {
	return publish(ctx, p.client, channelSignalDetected, event)
}

func (p *RedisPublisher) PublishSignalMapped(ctx context.Context, event SignalMappedEvent) error {
	return publish(ctx, p.client, channelSignalMapped, event)
}

func (p *RedisPublisher) PublishNextQuestionIndicated(ctx context.Context, event NextQuestionIndicatedEvent) error {
	return publish(ctx, p.client, channelNextQuestionIndicated, event)
}

func (p *RedisPublisher) PublishNextQuestionExtended(ctx context.Context, event NextQuestionExtendedEvent) error {
	return publish(ctx, p.client, channelNextQuestionExtended, event)
}

func (p *RedisPublisher) Close() error {
	return p.client.Close()
}
