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
	QuestionBankCreate(ctx context.Context, interviewID string, questions []adkutils.QuestionBankQuestion) error
	QuestionBankFind(ctx context.Context, interviewID string) ([]adkutils.QuestionBankQuestion, error)
	QuestionBankeQuestionFind(ctx context.Context, interviewID string, questionID string) (*adkutils.QuestionBankQuestion, error)
	// session summary
	SessionSummaryCreate(ctx context.Context, sessionID string, summary *SessionSummary) error
	SessionSummaryAppendTranscribedQuestion(ctx context.Context, sessionID string, questionID string, transcribedQuestion string) error
	SessionSummaryAppendAnswer(ctx context.Context, sessionID string, questionID string, answer string) error
	SessionAnswerJudgmentCreate(ctx context.Context, sessionID string, questionID string, judgment *Judgment) error
	SessionSummaryAppendQuestion(ctx context.Context, sessionID string, question *adkutils.QuestionBankQuestion) error
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

func NewRedisCacheClient(redisUrl string, redisPassword string) *RedisCacheClient {
	return &RedisCacheClient{
		client: redis.NewClient(&redis.Options{
			Addr:     redisUrl,
			Password: redisPassword,
		}),
	}
}
