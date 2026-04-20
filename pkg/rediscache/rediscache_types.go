package rediscache

import (
	"tal_assistant/pkg/adkutils"
	"tal_assistant/pkg/workableclient"
)

const (
	questionBankKeyPrefix    = "qbank:"
	currentQuestionKeyPrefix = "current:"
	summaryKeyPrefix         = "summary:"
	agentResponsesKeyPrefix  = "agent_responses:"
	eventDataKeyPrefix       = "event:"
	sessionKeyPrefix         = "session:"
)

// QuestionAnswer is one node in the interview summary tree.
// Follow-up questions are nested directly under their parent.
type QuestionAnswer struct {
	Question            adkutils.QuestionBankQuestion `json:"question"`
	TranscribedQuestion string                        `json:"transcribed_question"`
	Answer              string                        `json:"answer"`
	Judgment            *Judgment                     `json:"judgment,omitempty"`
	Order               int                           `json:"order"`
	FollowupQuestion    *QuestionAnswer               `json:"followup_question,omitempty"`
}

// Judgment holds the evaluation result from the judging agent
type Judgment struct {
	Score           int      `json:"score"`
	Pass            bool     `json:"pass"`
	Strengths       []string `json:"strengths"`
	Weaknesses      []string `json:"weaknesses"`
	MissingKeywords []string `json:"missing_keywords"`
	Verdict         string   `json:"verdict"`
}

// InterviewSummary is the full ordered Q&A history stored for an interview.
type SessionSummary struct {
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

// Session holds all data related to a single interview session.
type Session struct {
	SessionID      string                          `json:"session_id"`
	EventID        string                          `json:"event_id"`
	EventData      *workableclient.EventFindResult `json:"event_data,omitempty"`
	QuestionBank   []adkutils.QuestionBankQuestion `json:"question_bank,omitempty"`
	SessionSummary *SessionSummary                 `json:"interview_summary,omitempty"`
	Transcription  string                          `json:"transcription,omitempty"`
	Status         string                          `json:"status"` // "initialized", "in_progress", "completed", "cancelled"
	RecordingPath  string                          `json:"recording_path,omitempty"`
	StartedAt      int64                           `json:"started_at,omitempty"`   // Unix timestamp in milliseconds
	CompletedAt    int64                           `json:"completed_at,omitempty"` // Unix timestamp in milliseconds
	CreatedAt      int64                           `json:"created_at"`             // Unix timestamp in milliseconds
	UpdatedAt      int64                           `json:"updated_at"`             // Unix timestamp in milliseconds
}

type Event struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SessionSummaryFindResponse struct {
	InterviewID       string                          `json:"interview_id"`
	Questions         []adkutils.QuestionBankQuestion `json:"questions"`
	AppendedQuestions []adkutils.QuestionBankQuestion `json:"appended_questions"`
	Answers           map[string]string               `json:"answers"` // questionID → answer
	Judgments         []Judgment                      `json:"judgments"`
	DriveFolderURL    string                          `json:"drive_folder_url,omitempty"`
}
