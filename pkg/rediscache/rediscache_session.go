package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ── Key builders ───────────────────────────────────────────────────────────────

// sessionKey returns the Redis key for the core Session struct.
// The session metadata (status, timestamps, recording path) lives here.
//
//	Format: session:<sessionID>
//	Example: session:ses_abc123
func sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// sessionQuestionKey returns the Redis hash key that holds all transcribed
// questions asked during a session. Each field is a questionID; its value is
// the transcribed text of the question as it was spoken.
//
//	Format: session:<sessionID>:questions
//	Example: session:ses_abc123:questions
func sessionQuestionKey(sessionID string) string {
	return fmt.Sprintf("session:%s:questions", sessionID)
}

// sessionAnswerKey returns the Redis hash key that holds all candidate answers
// collected during a session. Each field is a questionID; its value is the
// raw answer text.
//
//	Format: session:<sessionID>:answers
//	Example: session:ses_abc123:answers
func sessionAnswerKey(sessionID string) string {
	return fmt.Sprintf("session:%s:answers", sessionID)
}

// sessionJudgmentKey returns the Redis hash key that holds judgment results
// produced by the judging agent. Each field is a questionID; its value is a
// JSON-encoded Judgment.
//
//	Format: session:<sessionID>:judgments
//	Example: session:ses_abc123:judgments
func sessionJudgmentKey(sessionID string) string {
	return fmt.Sprintf("session:%s:judgments", sessionID)
}

// ── Session ────────────────────────────────────────────────────────────────────

// SessionCreate persists a new Session. It serialises the full struct to JSON
// and stores it as a plain string value so the entire session can be retrieved
// and decoded in a single GET.
//
// Calling SessionCreate on an existing sessionID overwrites the previous value.
// Callers that need optimistic locking should check SessionFind first.
func (c *RedisCacheClient) SessionCreate(ctx context.Context, sessionID string, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session %s: %w", sessionID, err)
	}

	if err := c.client.Set(ctx, sessionKey(sessionID), string(data), 0).Err(); err != nil {
		return fmt.Errorf("write session %s: %w", sessionID, err)
	}

	return nil
}

// SessionFind retrieves a Session by its ID. Returns nil and no error when the
// session does not exist so callers can distinguish "not found" from a Redis
// failure without importing the redis package directly.
func (c *RedisCacheClient) SessionFind(ctx context.Context, sessionID string) (*Session, error) {
	raw, err := c.client.Get(ctx, sessionKey(sessionID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch session %s: %w", sessionID, err)
	}

	var session Session
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, fmt.Errorf("unmarshal session %s: %w", sessionID, err)
	}

	return &session, nil
}

// ── Session activity writers ───────────────────────────────────────────────────

// SessionQuestionCreate records the transcribed form of a question that was
// asked during the session. The questionID links this transcription back to the
// originating QuestionBankQuestion so callers can correlate questions, answers,
// and judgments by the same key.
//
// Writing the transcribed text separately from the question bank entry allows
// the system to capture how the question was actually phrased aloud, which may
// differ from the original bank text.
func (c *RedisCacheClient) SessionQuestionCreate(ctx context.Context, sessionID string, questionID string, transcribedQuestion string) error {
	if err := c.client.HSet(ctx, sessionQuestionKey(sessionID), questionID, transcribedQuestion).Err(); err != nil {
		return fmt.Errorf("write transcribed question %s for session %s: %w", questionID, sessionID, err)
	}

	return nil
}

// SessionAnswerCreate records the candidate's answer to a specific question.
// Answers are stored as raw text without processing so the judging agent
// receives the original response unmodified.
func (c *RedisCacheClient) SessionAnswerCreate(ctx context.Context, sessionID string, questionID string, answer string) error {
	if err := c.client.HSet(ctx, sessionAnswerKey(sessionID), questionID, answer).Err(); err != nil {
		return fmt.Errorf("write answer for question %s in session %s: %w", questionID, sessionID, err)
	}

	return nil
}

// SessionAnswerJudgmentCreate persists the judgment produced by the judging
// agent for a single question. Judgments are JSON-encoded so all scoring fields
// (score, pass/fail, strengths, weaknesses, missing keywords, verdict) survive
// the round-trip intact.
func (c *RedisCacheClient) SessionAnswerJudgmentCreate(ctx context.Context, sessionID string, questionID string, judgment *Judgment) error {
	data, err := json.Marshal(judgment)
	if err != nil {
		return fmt.Errorf("marshal judgment for question %s in session %s: %w", questionID, sessionID, err)
	}

	if err := c.client.HSet(ctx, sessionJudgmentKey(sessionID), questionID, string(data)).Err(); err != nil {
		return fmt.Errorf("write judgment for question %s in session %s: %w", questionID, sessionID, err)
	}

	return nil
}

// ── Session summary ────────────────────────────────────────────────────────────

// SessionSummaryFind assembles a complete picture of a session by fetching the
// session metadata, all answers, and all judgments in a single pipeline
// round-trip, then resolving the full QuestionBankQuestion structs for each
// answered question.
//
// The questions slice in the response is ordered by QuestionBankQuestion.Order
// and contains only questions for which an answer exists, so the caller always
// receives a coherent question-answer-judgment set with no dangling entries.
//
// Returns nil and no error when the session does not exist.
func (c *RedisCacheClient) SessionSummaryFind(ctx context.Context, sessionID string) (*SessionSummaryFindResponse, error) {
	// ── 1. Fetch session, answers, and judgments in one round-trip ─────────────
	pipe := c.client.Pipeline()
	sessionCmd := pipe.Get(ctx, sessionKey(sessionID))
	answersCmd := pipe.HGetAll(ctx, sessionAnswerKey(sessionID))
	judgmentsCmd := pipe.HGetAll(ctx, sessionJudgmentKey(sessionID))

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("fetch summary pipeline for session %s: %w", sessionID, err)
	}

	// ── 2. Resolve session (distinguishes not-found from pipeline error) ───────
	rawSession, err := sessionCmd.Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch session %s: %w", sessionID, err)
	}

	var session Session
	if err := json.Unmarshal([]byte(rawSession), &session); err != nil {
		return nil, fmt.Errorf("unmarshal session %s: %w", sessionID, err)
	}

	// ── 3. Collect answers ─────────────────────────────────────────────────────
	answers, err := answersCmd.Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("fetch answers for session %s: %w", sessionID, err)
	}

	// ── 4. Decode judgments ────────────────────────────────────────────────────
	rawJudgments, err := judgmentsCmd.Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("fetch judgments for session %s: %w", sessionID, err)
	}

	judgments := make([]Judgment, 0, len(rawJudgments))
	for questionID, raw := range rawJudgments {
		var j Judgment
		if err := json.Unmarshal([]byte(raw), &j); err != nil {
			return nil, fmt.Errorf("unmarshal judgment for question %s in session %s: %w", questionID, sessionID, err)
		}
		judgments = append(judgments, j)
	}

	// ── 5. Resolve full question structs for every answered question ───────────
	// Questions are fetched individually here because SessionSummaryFind does
	// not receive qbankID directly — it derives the needed questions from the
	// answer map, which is keyed by questionID. If your call-site has qbankID
	// available, consider passing it in and using QuestionBankGet instead for
	// a single HGETALL rather than N individual HGETs.
	questions := make([]adkutils.QuestionBankQuestion, 0, len(answers))
	for questionID := range answers {
		q, err := c.QuestionBankGetByQuestionID(ctx, session.EventID, session.QBankID, questionID)
		if err != nil {
			return nil, fmt.Errorf("resolve question %s for session %s: %w", questionID, sessionID, err)
		}
		if q != nil {
			questions = append(questions, *q)
		}
	}

	// Restore original question order so the summary is presented consistently.
	sort.Slice(questions, func(i int, j int) bool {
		return questions[i].Order < questions[j].Order
	})

	return &SessionSummaryFindResponse{
		EventID:   session.EventID,
		Questions: questions,
		Answers:   answers,
		Judgments: judgments,
	}, nil
}
