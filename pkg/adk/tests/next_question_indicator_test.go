package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"tal_assistant/pkg/adk/nextquestionindicator"
	"tal_assistant/pkg/adkutils"
	"testing"
)

var (
	testQuestion1 = adkutils.QuestionBankQuestion{
		ID:                   "TLQ001",
		Category:             "System Design",
		Difficulty:           "Hard",
		EstimatedTimeMinutes: 10,
		EvaluationCriteria: []adkutils.EvaluationCriteria{
			{
				BonusPoints: "edge inference\npartial transcript buffering\nfallback degradation strategy",
				MustMention: "real-time streaming\nlatency budget breakdown\nLLM call optimization",
			},
		},
		FollowupTriggers: []adkutils.FollowupTrigger{
			{Condition: "if_mentions_WebSocket", FollowUp: "How would you handle reconnection and message ordering if the WebSocket drops mid-interview?"},
			{Condition: "if_mentions_LLM", FollowUp: "How do you keep LLM inference under 500ms for the tip generation step?"},
			{Condition: "if_skips_VAD", FollowUp: "How does the system know when the candidate has finished speaking before sending to the LLM?"},
		},
		IdealAnswerKeywords: "WebSocket\nVAD\nstreaming transcription\nmessage queue\nLLM inference\nlatency budget\nRedis\nevent-driven",
		PassThreshold:       0.7,
		Question:            "Design a real-time interview assistant system that processes live audio, extracts Q&A pairs, and surfaces tips to the recruiter with under 2 seconds of latency. Walk me through the architecture.",
	}

	testQuestion2 = adkutils.QuestionBankQuestion{
		ID:                   "TLQ002",
		Category:             "Programming",
		Difficulty:           "Medium",
		EstimatedTimeMinutes: 5,
		IdealAnswerKeywords:  "GIL\nCPython\nmutex\nmultiprocessing\nthread",
		PassThreshold:        0.6,
		Question:             "Explain the Python GIL and its impact on multi-threaded programs.",
	}

	testNextQuestionIndicatorState = nextquestionindicator.NextQuestionIndicatorState{
		QuestionBank: []adkutils.QuestionBankQuestion{testQuestion1, testQuestion2},
	}
)

func TestNextQuestionIndicatorRun(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	state := svc.NewNextQuestionIndicatorState(testNextQuestionIndicatorState)
	if err := svc.SessionUpsert(ctx, id, testUserName, state); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	cases := []struct {
		name            string
		currentQuestion adkutils.QuestionBankQuestion
		answer          string
	}{
		{
			name:            "strong_answer_triggers_next_question",
			currentQuestion: testQuestion1,
			answer:          `I'd use a WebSocket connection for real-time audio streaming from the client. Audio chunks go through a VAD (Voice Activity Detection) module to detect speech boundaries, then get sent to a streaming transcription service. Transcripts are pushed onto a Redis message queue. A consumer picks them up, calls an LLM inference service for tip generation — we'd optimize LLM calls to stay within a 500ms latency budget — and the results are pushed back over WebSocket to the recruiter UI. The whole pipeline is event-driven to keep end-to-end latency under 2 seconds.`,
		},
		{
			name:            "weak_answer_no_action",
			currentQuestion: testQuestion1,
			answer:          `I would use a microphone and send the audio to a server somehow.`,
		},
		{
			name:            "answer_mentions_websocket_triggers_followup",
			currentQuestion: testQuestion1,
			answer:          `I'd use a WebSocket to stream audio from the browser to the backend, then process it there.`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Send current question (agent should not respond)
			questionJSON, _ := json.MarshalIndent(tc.currentQuestion, "", "  ")
			questionPrompt := fmt.Sprintf("Current Question Entity:\n%s", string(questionJSON))
			
			for _, err := range svc.NextQuestionIndicatorRun(adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    questionPrompt,
			}) {
				if err != nil {
					t.Fatalf("error sending question: %v", err)
				}
				// Consume and discard any output (agent should not respond)
			}
			
			// Step 2: Send candidate's answer (agent should respond with decision)
			answerPrompt := fmt.Sprintf("Candidate's Answer:\n%s", tc.answer)
			var sb strings.Builder
			for chunk, err := range svc.NextQuestionIndicatorRun(adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    answerPrompt,
			}) {
				if err != nil {
					t.Errorf("NextQuestionIndicatorRun: %v", err)
					break
				}
				sb.WriteString(chunk)
			}
			output := strings.TrimSpace(sb.String())
			
			// Record the full Q&A exchange and the decision
			input := fmt.Sprintf("Q: %s\nA: %s", tc.currentQuestion.Question, tc.answer)
			writeRecord(t, id, input, output)

			valid := output == "None" ||
				strings.HasPrefix(output, "F:") ||
				strings.HasPrefix(output, "C:")
			if !valid {
				t.Errorf("unexpected output format: %q", output)
			}
			t.Logf("case %q => %s", tc.name, output)
		})
	}
}
