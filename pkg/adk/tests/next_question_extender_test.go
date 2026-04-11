package tests

import (
	"context"
	"strings"
	"tal_assistant/pkg/adk/nextquestionextender"
	"tal_assistant/pkg/adkutils"
	"testing"
)

var testNextQuestionExtenderState = nextquestionextender.NextQuestionExtenderState{
	QuestionBank: []adkutils.QuestionBankQuestion{testQuestion1, testQuestion2},
}

func TestNextQuestionExtenderRun(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	state := svc.NewNextQuestionExtenderState(testNextQuestionExtenderState)
	if err := svc.SessionUpsert(ctx, id, testUserName, state); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	cases := []struct {
		name             string
		questionText     string
		parentQuestionID string
	}{
		{
			name:             "followup_question_with_parent",
			questionText:     "How would you handle reconnection and message ordering if the WebSocket drops mid-interview?",
			parentQuestionID: "TLQ001",
		},
		{
			name:         "change_question_no_parent",
			questionText: "How does Go handle concurrency differently from Python's threading model?",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt: nextquestionextender.NextQuestionExtenderInput{
					QuestionText:     tc.questionText,
					ParentQuestionID: tc.parentQuestionID,
				},
			}
			question, err := svc.NextQuestionExtenderRun(req)
			if err != nil {
				t.Fatalf("NextQuestionExtenderRun: %v", err)
			}

			writeRecord(t, id, tc.questionText, question.ID)

			if question.ID == "" {
				t.Error("expected non-empty Name")
			}
			if !strings.HasPrefix(question.ID, "TLQ") {
				t.Errorf("expected Name to start with TLQ, got %q", question.ID)
			}
			if question.Question != tc.questionText {
				t.Errorf("expected Question %q, got %q", tc.questionText, question.Question)
			}
			if question.PassThreshold == 0 {
				t.Error("expected non-zero PassThreshold")
			}
			t.Logf("case %q => name=%s category=%s difficulty=%s", tc.name, question.ID, question.Category, question.Difficulty)
		})
	}
}
