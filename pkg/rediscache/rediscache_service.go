package rediscache

import (
	"context"
	"tal_assistant/pkg/adkutils"

	"github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────
// Interface
// ─────────────────────────────────────────────

type RedisCacheInterface interface {
	//event
	EventCreate(ctx context.Context, eventID string, event *Event) error
	EventFind(ctx context.Context, eventID string) (*Event, error)

	//session
	SessionCreate(ctx context.Context, sessionID string, session *Session) error
	SessionFind(ctx context.Context, sessionID string) (*Session, error)

	//question bank
	QuestionBankCreate(
		ctx context.Context,
		interviewID string,
		questions []adkutils.QuestionBankQuestion,
		embedFunc func(text string) ([]float32, error),
	) (string, error)
	QuestionBankFind(ctx context.Context, eventID string, qbankID string) ([]adkutils.QuestionBankQuestion, error)
	QuestionBankSearch(
		ctx context.Context,
		interviewID, qbankID, query string,
		threshold float64,
		embedFunc func(text string) ([]float32, error),
	) ([]adkutils.QuestionBankQuestion, error)
	QuestionBankQuestionFind(
		ctx context.Context,
		eventID string,
		qbankID string,
		questionID string,
	) (*adkutils.QuestionBankQuestion, error)
	// session summary
	// SessionSummaryCreate(ctx context.Context, sessionID string, summary *SessionSummary) error
	SessionQuestionCreate(ctx context.Context, sessionID string, questionID string, transcribedQuestion string) error
	SessionAnswerCreate(ctx context.Context, sessionID string, questionID string, answer string) error
	SessionAnswerJudgmentCreate(ctx context.Context, sessionID string, questionID string, judgment *Judgment) error
	SessionSummaryFind(ctx context.Context, sessionID string) (*SessionSummaryFindResponse, error)

	// agent response
	AgentResponseCreate(ctx context.Context, sessionID string, response *AgentResponse) error
}

// ─────────────────────────────────────────────
// Implementation
// ─────────────────────────────────────────────

type RedisCacheClient struct {
	client *redis.Client
}

func NewRedisCacheClient(redisUrl string, redisPassword string) RedisCacheInterface {
	return &RedisCacheClient{
		client: redis.NewClient(&redis.Options{
			Addr:     redisUrl,
			Password: redisPassword,
		}),
	}
}
