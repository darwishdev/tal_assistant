package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tal_assistant/pkg/adk"
	"tal_assistant/pkg/adkutils"
	redispkg "tal_assistant/pkg/redis"
	"testing"
	"time"
)

// ─────────────────────────────────────────────
// Mock data
// ─────────────────────────────────────────────

var mockQuestionBank = []adkutils.QuestionBankQuestion{
	{
		ID:                   "TLQ001",
		Category:             "System Design",
		Difficulty:           "Hard",
		EstimatedTimeMinutes: 10,
		EvaluationCriteria: []adkutils.EvaluationCriteria{
			{
				MustMention: "real-time streaming\nlatency budget\nLLM call optimization",
				BonusPoints: "edge inference\npartial transcript buffering",
			},
		},
		FollowupTriggers: []adkutils.FollowupTrigger{
			{Condition: "if_mentions_WebSocket", FollowUp: "How would you handle reconnection if the WebSocket drops mid-interview?"},
			{Condition: "if_mentions_LLM", FollowUp: "How do you keep LLM inference under 500ms?"},
			{Condition: "if_skips_VAD", FollowUp: "How does the system know when the candidate has finished speaking?"},
		},
		IdealAnswerKeywords: "WebSocket\nVAD\nstreaming transcription\nRedis\nLLM inference\nlatency budget",
		PassThreshold:       0.7,
		Question:            "Design a real-time interview assistant that processes live audio and surfaces tips to the recruiter under 2 seconds of latency.",
	},
	{
		ID:                   "TLQ002",
		Category:             "Programming",
		Difficulty:           "Medium",
		EstimatedTimeMinutes: 5,
		EvaluationCriteria: []adkutils.EvaluationCriteria{
			{
				MustMention: "GIL\nCPython\nmutex\nthread",
				BonusPoints: "multiprocessing\nasync IO",
			},
		},
		IdealAnswerKeywords: "GIL\nCPython\nmutex\nmultiprocessing\nthread safety",
		PassThreshold:       0.6,
		Question:            "Explain the Python GIL and its impact on multi-threaded programs.",
	},
}

// mockTranscript simulates a realistic interview conversation turn by turn.
var mockTranscript = []string{
	"Recruiter: Design a real-time interview assistant that processes live audio and surfaces tips to the recruiter under 2 seconds of latency. Walk me through the architecture.",
	"Candidate: I would use a WebSocket connection to stream audio chunks from the client in real time. Each chunk goes through VAD — voice activity detection — to detect when the candidate finishes speaking. Once a speech boundary is detected, the audio is sent to a streaming transcription service. The transcript is pushed onto a Redis message queue. A consumer picks it up, calls an LLM with a tight latency budget of around 500ms for tip generation, and the result is pushed back to the recruiter UI over the same WebSocket. The entire pipeline is event-driven to keep end-to-end latency under 2 seconds.",
	"Recruiter: Explain the Python GIL and its impact on multi-threaded programs.",
	"Candidate: The GIL is a mutex in CPython that allows only one thread to execute Python bytecode at a time, which prevents true parallelism for CPU-bound threads. For IO-bound work the GIL is released during IO waits, so threading still helps there. For CPU-bound parallelism I would use the multiprocessing module instead, which spawns separate processes each with their own GIL.",
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func skipIfNoAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		t.Skip("GOOGLE_API_KEY not set — skipping integration test")
	}
	return key
}

func writeSummaryFile(
	t *testing.T,
	interviewID string,
	summary *redispkg.InterviewSummary,
	agentResponses []redispkg.AgentResponse,
) {
	t.Helper()

	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}

	path := filepath.Join("testdata", fmt.Sprintf("summary_%s.txt", interviewID))
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create summary file: %v", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "Interview Summary\n")
	fmt.Fprintf(f, "interview_id : %s\n", summary.InterviewID)
	fmt.Fprintf(f, "timestamp    : %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "════════════════════════════════════════\n\n")

	fmt.Fprintf(f, "Q&A\n")
	fmt.Fprintf(f, "────────────────────────────────────────\n")
	for _, qa := range summary.Questions {
		writeQuestionAnswer(f, qa, 0)
	}

	fmt.Fprintf(f, "════════════════════════════════════════\n")
	fmt.Fprintf(f, "Agent Responses (%d)\n", len(agentResponses))
	fmt.Fprintf(f, "────────────────────────────────────────\n")
	for _, r := range agentResponses {
		fmt.Fprintf(f, "  [%s] agent=%s\n",
			time.UnixMilli(r.Timestamp).Format("15:04:05.000"),
			r.Agent,
		)
		fmt.Fprintf(f, "    input  : %s\n", r.Input)
		fmt.Fprintf(f, "    output : %s\n", r.Output)
		fmt.Fprintln(f)
	}

	t.Logf("summary written to %s", path)
}

func writeQuestionAnswer(f *os.File, qa redispkg.QuestionAnswer, depth int) {
	indent := strings.Repeat("  ", depth)
	label := "Question"
	if depth > 0 {
		label = "Follow-up"
	}
	fmt.Fprintf(f, "%s[%s #%d — %s]\n", indent, label, qa.Order, qa.Question.ID)
	fmt.Fprintf(f, "%sBank question    : %s\n", indent, qa.Question.Question)
	fmt.Fprintf(f, "%sTranscribed      : %s\n", indent, qa.TranscribedQuestion)
	fmt.Fprintf(f, "%sAnswer           : %s\n", indent, qa.Answer)
	if qa.FollowupQuestion != nil {
		writeQuestionAnswer(f, *qa.FollowupQuestion, depth+1)
	}
	fmt.Fprintln(f)
}

// ─────────────────────────────────────────────
// Test
// ─────────────────────────────────────────────

func TestOrchestratorIntegration(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// ── wire up services ──────────────────────
	adkSvc, err := adk.NewADKService(ctx, apiKey)
	if err != nil {
		t.Fatalf("NewADKService: %v", err)
	}

	cache := redispkg.NewRedisCacheClient("")
	publisher := redispkg.NewRedisPublisher("")
	emitToUi := func(event string, data interface{}) {
		fmt.Printf("emitted to ui event :%s , data : %v", event, data)
	}
	subscriber := redispkg.NewOrchestrationSubscriber("", adkSvc, publisher, cache, emitToUi)

	// ── unique IDs for this run ───────────────
	interviewID := fmt.Sprintf("test_%d", time.Now().UnixMilli())
	userID := "test_user"

	// ── start subscriber ──────────────────────
	go subscriber.Run(ctx)
	time.Sleep(200 * time.Millisecond) // let subscriber connect

	// ── start ADK sessions ────────────────────
	sessions, err := adkSvc.StartSession(ctx, userID, mockQuestionBank)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// ── init redis data ───────────────────────
	if err := cache.SaveQuestionBank(ctx, interviewID, mockQuestionBank); err != nil {
		t.Fatalf("SaveQuestionBank: %v", err)
	}
	if err := cache.InitInterviewSummary(ctx, interviewID, mockQuestionBank); err != nil {
		t.Fatalf("InitInterviewSummary: %v", err)
	}

	// ── process transcript ────────────────────
	// accumulate Q&A per question for NQI context
	var qandaBuf strings.Builder

	for i, line := range mockTranscript {
		t.Logf("turn %d: %s", i+1, line)

		// run signaling agent to extract signal
		var sigBuf strings.Builder
		for chunk, err := range adkSvc.SignalingAgentRun(adkutils.AgentRunRequest{
			Ctx:       ctx,
			SessionID: sessions.SignalingAgentSessionID,
			UserID:    userID,
			Prompt:    line,
		}) {
			if err != nil {
				t.Logf("signaling agent error on turn %d: %v", i+1, err)
				break
			}
			emitToUi("signal_chunk_detected", chunk)
			sigBuf.WriteString(chunk)
		}
		signal := strings.TrimSpace(sigBuf.String())
		t.Logf("  signal: %q", signal)

		if signal == "" || signal == "UNCLEAR" {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// accumulate Q&A context (reset on new Q:, append on A:)
		if strings.HasPrefix(signal, "Q:") {
			qandaBuf.Reset()
		}
		qandaBuf.WriteString(signal)
		qandaBuf.WriteString("\n")

		if err := publisher.PublishSignalDetected(ctx, redispkg.SignalDetectedEvent{
			InterviewID:        interviewID,
			UserID:             userID,
			TranscriptLine:     line,
			Signal:             signal,
			QAndA:              qandaBuf.String(),
			SignalingSessionID: sessions.SignalingAgentSessionID,
			MapperSessionID:    sessions.SignalingAgentMapperSessionID,
			IndicatorSessionID: sessions.NextQuestionIndicatorSessionID,
			ExtenderSessionID:  sessions.NextQuestionExtenderSessionID,
			JudgingSessionID:   sessions.JudgingAgentSessionID,
		}); err != nil {
			t.Fatalf("PublishSignalDetected turn %d: %v", i+1, err)
		}

		// give the pipeline time to process before the next turn
		time.Sleep(5 * time.Second)
	}

	// allow the last pipeline run to fully complete
	t.Log("waiting for final pipeline completion...")
	time.Sleep(10 * time.Second)

	// ── fetch and write summary ───────────────
	summary, err := cache.FindInterviewSummary(ctx, interviewID)
	if err != nil {
		t.Fatalf("FindInterviewSummary: %v", err)
	}

	agentResponses, err := cache.FindAgentResponses(ctx, interviewID)
	if err != nil {
		t.Fatalf("FindAgentResponses: %v", err)
	}

	writeSummaryFile(t, interviewID, summary, agentResponses)

	// log summary JSON for CI visibility
	raw, _ := json.MarshalIndent(summary, "", "  ")
	t.Logf("final summary:\n%s", string(raw))
}
