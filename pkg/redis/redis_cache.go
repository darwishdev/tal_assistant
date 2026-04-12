package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"tal_assistant/pkg/adkutils"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	questionBankKeyPrefix    = "qbank:"
	currentQuestionKeyPrefix = "current:"
	summaryKeyPrefix         = "summary:"
	agentResponsesKeyPrefix  = "agent_responses:"
)

// QuestionAnswer is one node in the interview summary tree.
// Follow-up questions are nested directly under their parent.
type QuestionAnswer struct {
	Question            adkutils.QuestionBankQuestion `json:"question"`
	TranscribedQuestion string                        `json:"transcribed_question"`
	Answer              string                        `json:"answer"`
	Order               int                           `json:"order"`
	FollowupQuestion    *QuestionAnswer               `json:"followup_question,omitempty"`
}

// InterviewSummary is the full ordered Q&A history stored for an interview.
type InterviewSummary struct {
	InterviewID string           `json:"interview_id"`
	Questions   []QuestionAnswer `json:"questions"`
}

// AgentResponse records one input/output pair for any agent in the pipeline.
type AgentResponse struct {
	Agent     string `json:"agent"`
	Input     string `json:"input"`
	Output    string `json:"output"`
	Timestamp int64  `json:"timestamp"`
}

// ─────────────────────────────────────────────
// Interface
// ─────────────────────────────────────────────

type RedisCacheInterface interface {
	// Question bank — lookup map for agents
	SaveQuestionBank(ctx context.Context, interviewID string, questions []adkutils.QuestionBankQuestion) error
	FindQuestionBank(ctx context.Context, interviewID string) (map[string]adkutils.QuestionBankQuestion, error)
	FindQuestionByID(ctx context.Context, interviewID string, questionID string) (*adkutils.QuestionBankQuestion, error)

	// Current question pointer
	UpsertCurrentQuestionPointer(ctx context.Context, interviewID string, questionID string) error
	FindCurrentQuestionPointer(ctx context.Context, interviewID string) (string, error)

	// Interview summary
	InitInterviewSummary(ctx context.Context, interviewID string, questions []adkutils.QuestionBankQuestion) error
	SaveTranscribedQuestion(ctx context.Context, interviewID string, questionID string, transcribedQuestion string) error
	SaveAnswer(ctx context.Context, interviewID string, questionID string, answer string) error
	InsertFollowUpQuestion(ctx context.Context, interviewID string, parentQuestionID string, followUp adkutils.QuestionBankQuestion) error
	SaveChangeQuestion(ctx context.Context, interviewID string, question adkutils.QuestionBankQuestion) error
	FindInterviewSummary(ctx context.Context, interviewID string) (*InterviewSummary, error)

	// Agent responses
	SaveAgentResponse(ctx context.Context, interviewID string, response AgentResponse) error
	FindAgentResponses(ctx context.Context, interviewID string) ([]AgentResponse, error)
}

// ─────────────────────────────────────────────
// Implementation
// ─────────────────────────────────────────────

type RedisCacheClient struct {
	client *redis.Client
}

func NewRedisCacheClient() *RedisCacheClient {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	return &RedisCacheClient{
		client: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

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
	for _, q := range questions {
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

// FindQuestionBank returns every question in the bank as a map keyed by question ID.
func (c *RedisCacheClient) FindQuestionBank(
	ctx context.Context,
	interviewID string,
) (map[string]adkutils.QuestionBankQuestion, error) {
	key := questionBankKeyPrefix + interviewID
	raw, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("find question bank for interview %s: %w", interviewID, err)
	}
	result := make(map[string]adkutils.QuestionBankQuestion, len(raw))
	for questionID, data := range raw {
		var q adkutils.QuestionBankQuestion
		if err := json.Unmarshal([]byte(data), &q); err != nil {
			return nil, fmt.Errorf("unmarshal question %s: %w", questionID, err)
		}
		result[questionID] = q
	}
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

func (c *RedisCacheClient) FindCurrentQuestionPointer(
	ctx context.Context,
	interviewID string,
) (string, error) {
	key := currentQuestionKeyPrefix + interviewID
	questionID, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("find current question pointer for interview %s: %w", interviewID, err)
	}
	return questionID, nil
}

// ── Interview summary ──────────────────────────

func (c *RedisCacheClient) summaryKey(interviewID string) string {
	return summaryKeyPrefix + interviewID
}

func (c *RedisCacheClient) loadSummary(ctx context.Context, interviewID string) (*InterviewSummary, error) {
	raw, err := c.client.Get(ctx, c.summaryKey(interviewID)).Result()
	if err != nil {
		return nil, fmt.Errorf("load summary for interview %s: %w", interviewID, err)
	}
	var summary InterviewSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return nil, fmt.Errorf("unmarshal summary for interview %s: %w", interviewID, err)
	}
	return &summary, nil
}

func (c *RedisCacheClient) saveSummary(ctx context.Context, summary *InterviewSummary) error {
	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal summary for interview %s: %w", summary.InterviewID, err)
	}
	if err := c.client.Set(ctx, c.summaryKey(summary.InterviewID), data, 0).Err(); err != nil {
		return fmt.Errorf("save summary for interview %s: %w", summary.InterviewID, err)
	}
	return nil
}

// findNode searches the summary tree for the QuestionAnswer with the given questionID.
// It returns a pointer to the node so the caller can mutate it in place.
func findNode(questions []QuestionAnswer, questionID string) *QuestionAnswer {
	for i := range questions {
		if questions[i].Question.ID == questionID {
			return &questions[i]
		}
		if questions[i].FollowupQuestion != nil {
			if found := findNodeInChain(questions[i].FollowupQuestion, questionID); found != nil {
				return found
			}
		}
	}
	return nil
}

func findNodeInChain(qa *QuestionAnswer, questionID string) *QuestionAnswer {
	if qa.Question.ID == questionID {
		return qa
	}
	if qa.FollowupQuestion != nil {
		return findNodeInChain(qa.FollowupQuestion, questionID)
	}
	return nil
}

// tailOfChain walks the follow-up chain and returns the last node (the one with no follow-up yet).
func tailOfChain(qa *QuestionAnswer) *QuestionAnswer {
	if qa.FollowupQuestion == nil {
		return qa
	}
	return tailOfChain(qa.FollowupQuestion)
}

// InitInterviewSummary creates the ordered summary skeleton from the initial question bank.
// Must be called once at the start of the interview.
func (c *RedisCacheClient) InitInterviewSummary(
	ctx context.Context,
	interviewID string,
	questions []adkutils.QuestionBankQuestion,
) error {
	entries := make([]QuestionAnswer, len(questions))
	for i, q := range questions {
		entries[i] = QuestionAnswer{
			Question: q,
			Order:    i + 1,
		}
	}
	return c.saveSummary(ctx, &InterviewSummary{
		InterviewID: interviewID,
		Questions:   entries,
	})
}

// SaveTranscribedQuestion stores the actual spoken question text for a question.
func (c *RedisCacheClient) SaveTranscribedQuestion(
	ctx context.Context,
	interviewID string,
	questionID string,
	transcribedQuestion string,
) error {
	summary, err := c.loadSummary(ctx, interviewID)
	if err != nil {
		return err
	}
	node := findNode(summary.Questions, questionID)
	if node == nil {
		return fmt.Errorf("question %s not found in summary for interview %s", questionID, interviewID)
	}
	node.TranscribedQuestion = transcribedQuestion
	return c.saveSummary(ctx, summary)
}

// SaveAnswer stores the candidate's answer for a question.
func (c *RedisCacheClient) SaveAnswer(
	ctx context.Context,
	interviewID string,
	questionID string,
	answer string,
) error {
	summary, err := c.loadSummary(ctx, interviewID)
	if err != nil {
		return err
	}
	node := findNode(summary.Questions, questionID)
	if node == nil {
		return fmt.Errorf("question %s not found in summary for interview %s", questionID, interviewID)
	}
	node.Answer = answer
	return c.saveSummary(ctx, summary)
}

// InsertFollowUpQuestion attaches a follow-up question at the tail of the parent's follow-up chain.
// This also adds the follow-up to the question bank hash so agents can look it up.
func (c *RedisCacheClient) InsertFollowUpQuestion(
	ctx context.Context,
	interviewID string,
	parentQuestionID string,
	followUp adkutils.QuestionBankQuestion,
) error {
	// persist to question bank for agent lookups
	if err := c.SaveQuestionBank(ctx, interviewID, []adkutils.QuestionBankQuestion{followUp}); err != nil {
		return err
	}

	summary, err := c.loadSummary(ctx, interviewID)
	if err != nil {
		return err
	}
	parent := findNode(summary.Questions, parentQuestionID)
	if parent == nil {
		return fmt.Errorf("parent question %s not found in summary for interview %s", parentQuestionID, interviewID)
	}
	tail := tailOfChain(parent)
	tail.FollowupQuestion = &QuestionAnswer{
		Question: followUp,
		Order:    tail.Order,
	}
	return c.saveSummary(ctx, summary)
}

// SaveChangeQuestion appends a new question at the end of the root-level question list.
// This also adds it to the question bank hash.
func (c *RedisCacheClient) SaveChangeQuestion(
	ctx context.Context,
	interviewID string,
	question adkutils.QuestionBankQuestion,
) error {
	if err := c.SaveQuestionBank(ctx, interviewID, []adkutils.QuestionBankQuestion{question}); err != nil {
		return err
	}

	summary, err := c.loadSummary(ctx, interviewID)
	if err != nil {
		return err
	}
	nextOrder := len(summary.Questions) + 1
	summary.Questions = append(summary.Questions, QuestionAnswer{
		Question: question,
		Order:    nextOrder,
	})
	return c.saveSummary(ctx, summary)
}

// FindInterviewSummary returns the full ordered Q&A summary for an interview.
func (c *RedisCacheClient) FindInterviewSummary(
	ctx context.Context,
	interviewID string,
) (*InterviewSummary, error) {
	return c.loadSummary(ctx, interviewID)
}

func (c *RedisCacheClient) SaveAgentResponse(
	ctx context.Context,
	interviewID string,
	response AgentResponse,
) error {
	if response.Timestamp == 0 {
		response.Timestamp = time.Now().UnixMilli()
	}
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal agent response: %w", err)
	}
	key := agentResponsesKeyPrefix + interviewID
	if err := c.client.RPush(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("save agent response for interview %s: %w", interviewID, err)
	}
	return nil
}

func (c *RedisCacheClient) FindAgentResponses(
	ctx context.Context,
	interviewID string,
) ([]AgentResponse, error) {
	key := agentResponsesKeyPrefix + interviewID
	raw, err := c.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("find agent responses for interview %s: %w", interviewID, err)
	}
	responses := make([]AgentResponse, 0, len(raw))
	for _, item := range raw {
		var r AgentResponse
		if err := json.Unmarshal([]byte(item), &r); err != nil {
			return nil, fmt.Errorf("unmarshal agent response: %w", err)
		}
		responses = append(responses, r)
	}
	return responses, nil
}

func (c *RedisCacheClient) Close() error {
	return c.client.Close()
}
