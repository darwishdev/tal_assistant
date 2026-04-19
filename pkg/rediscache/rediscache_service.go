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
	EventCreate(ctx context.Context, eventID string, event *DBEvent) error
	EventFind(ctx context.Context, eventID string) (*DBEvent, error)

	//session
	SessionCreate(ctx context.Context, sessionID string, session *Session) error
	SessionFind(ctx context.Context, sessionID string) (*Session, error)

	//question bank
	QuestionBankCreate(ctx context.Context, interviewID string, questions []adkutils.QuestionBankQuestion) error
	QuestionBankFind(ctx context.Context, interviewID string) ([]adkutils.QuestionBankQuestion, error)
	QuestionBankeQuestionFind(ctx context.Context, interviewID string, questionID string) (*adkutils.QuestionBankQuestion, error)

	// agent response
	AgentResponseCreate(ctx context.Context, sessionID string, response *AgentResponse) error

	SessionSummaryCreate(ctx context.Context, sessionID string, summary *SessionSummary) error
	SessionSummaryAppendTranscribedQuestion(ctx context.Context, sessionID string, questionID string, transcribedQuestion string) error
	SessionSummaryAppendAnswer(ctx context.Context, sessionID string, questionID string, answer string) error
	SessionAnswerJudgmentCreate(ctx context.Context, sessionID string, questionID string, judgment *Judgment) error
	SessionSummaryAppendQuestion(ctx context.Context, sessionID string, question *adkutils.QuestionBankQuestion) error
	SessionSummaryFind(ctx context.Context, sessionID string) (*SessionSummaryFindResponse, error)

	// Event data — Workable EventFindResult
	// SaveEventData(ctx context.Context, eventID string, event *workableclient.EventFindResult) error
	// FindEventData(ctx context.Context, eventID string) (*workableclient.EventFindResult, error)

	// // Question bank — lookup map for agents
	// SaveQuestionBank(ctx context.Context, interviewID string, questions []adkutils.QuestionBankQuestion) error
	// FindQuestionBank(ctx context.Context, interviewID string) ([]adkutils.QuestionBankQuestion, error)
	// FindQuestionByID(ctx context.Context, interviewID string, questionID string) (*adkutils.QuestionBankQuestion, error)

	// // Current question pointer
	// UpsertCurrentQuestionPointer(ctx context.Context, interviewID string, questionID string) error
	// FindCurrentQuestionPointer(ctx context.Context, interviewID string) (string, error)

	// // Interview summary
	// InitInterviewSummary(ctx context.Context, interviewID string, questions []adkutils.QuestionBankQuestion) error
	// SaveTranscribedQuestion(ctx context.Context, interviewID string, questionID string, transcribedQuestion string) error
	// SaveAnswer(ctx context.Context, interviewID string, questionID string, answer string) error
	// SaveJudgment(ctx context.Context, interviewID string, questionID string, judgment *Judgment) error
	// InsertFollowUpQuestion(ctx context.Context, interviewID string, parentQuestionID string, followUp adkutils.QuestionBankQuestion) error
	// SaveChangeQuestion(ctx context.Context, interviewID string, question adkutils.QuestionBankQuestion) error
	// FindInterviewSummary(ctx context.Context, interviewID string) (*InterviewSummary, error)

	// // Agent responses
	// SaveAgentResponse(ctx context.Context, interviewID string, response AgentResponse) error
	// FindAgentResponses(ctx context.Context, interviewID string) ([]AgentResponse, error)

	// // Session management
	// SaveSession(ctx context.Context, session *Session) error
	// FindSession(ctx context.Context, sessionID string) (*Session, error)
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
