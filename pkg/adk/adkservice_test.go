package adk

// Integration tests for InterviewService / SignalingAgentRunner.
//
// These tests exercise ONLY the two interfaces:
//   - InterviewService.SignalingAgent()
//   - SignalingAgentRunner.StartSession()
//   - SignalingAgentRunner.SendTurn()
//
// They use a real Gemini model. Every test writes its full input and the
// collected model output to testdata/<TestName>.txt so you can inspect
// exactly what the model received and returned.
//
// Run:
//   GOOGLE_API_KEY=<key> go test ./pkg/adk/... -v

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// ─────────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────────

const testModel = "gemini-2.0-flash"

const testQuestionBank = `[
  {
    "id": "TLQ001",
    "category": "Python Advanced",
    "difficulty": "Medium",
    "question": "Explain the Python GIL and its impact on multi-threaded programs.",
    "evaluation_criteria": {
      "must_mention": ["GIL", "thread", "CPython"],
      "bonus_points":  ["asyncio", "multiprocessing"]
    },
    "pass_threshold": 0.65
  },
  {
    "id": "TLQ002",
    "category": "System Design",
    "difficulty": "Hard",
    "question": "Design a real-time leaderboard for a multiplayer game.",
    "evaluation_criteria": {
      "must_mention": ["Redis", "sorted set", "WebSocket"],
      "bonus_points":  ["sharding", "eventual consistency"]
    },
    "pass_threshold": 0.70
  }
]`

// newService builds an InterviewService backed by a real Gemini model.
// Skips the test if GOOGLE_API_KEY is not set.
func newService(t *testing.T) InterviewService {
	t.Helper()
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		t.Skip("GOOGLE_API_KEY not set — skipping integration test")
	}
	llm, err := gemini.NewModel(context.Background(), testModel, &genai.ClientConfig{
		APIKey: key,
	})
	if err != nil {
		t.Fatalf("gemini.NewModel: %v", err)
	}
	svc, err := NewADKService(llm)
	if err != nil {
		t.Fatalf("NewADKService: %v", err)
	}
	return svc
}

// collectTurn calls SendTurn via the interface and collects the full signal.
// It also writes a structured record of input → output to testdata/<name>.txt.
func collectTurn(
	t *testing.T,
	runner SignalingAgentRunner,
	sessionID, transcript string,
) string {
	t.Helper()

	var sb strings.Builder
	for chunk, err := range runner.SendTurn(context.Background(), sessionID, "test_user", transcript) {
		if err != nil {
			t.Fatalf("SendTurn error: %v", err)
		}
		sb.WriteString(chunk)
	}
	signal := strings.TrimSpace(sb.String())

	writeRecord(t, sessionID, transcript, signal)
	return signal
}

// writeRecord appends one input/output record to testdata/<TestName>.txt.
// Each call appends so multi-turn tests accumulate in a single file.
func writeRecord(t *testing.T, sessionID, input, output string) {
	t.Helper()

	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}

	name := strings.ReplaceAll(t.Name(), "/", "_")
	path := filepath.Join("testdata", name+".txt")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open record file: %v", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "────────────────────────────────────────\n")
	fmt.Fprintf(f, "test      : %s\n", t.Name())
	fmt.Fprintf(f, "timestamp : %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "session   : %s\n", sessionID)
	fmt.Fprintf(f, "input     : %s\n", input)
	fmt.Fprintf(f, "output    : %s\n", output)
	fmt.Fprintf(f, "\n")
}

// isValidSignal checks the three allowed signal shapes.
func isValidSignal(s string) bool {
	return strings.HasPrefix(s, "Q:") ||
		strings.HasPrefix(s, "A:") ||
		s == "UNCLEAR"
}

// uniqueID returns a simple unique session ID for each test run.
func uniqueID(t *testing.T) string {
	return strings.ReplaceAll(t.Name(), "/", "_") +
		"_" + fmt.Sprintf("%d", time.Now().UnixMilli())
}

// ─────────────────────────────────────────────
// 1. InterviewService returns a non-nil SignalingAgentRunner
// ─────────────────────────────────────────────

func TestInterviewService_SignalingAgentNotNil(t *testing.T) {
	svc := newService(t)

	runner := svc.SignalingAgent()
	if runner == nil {
		t.Fatal("SignalingAgent() returned nil")
	}
}

// ─────────────────────────────────────────────
// 2. StartSession — happy path
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_StartSession(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()

	err := runner.StartSession(context.Background(), uniqueID(t), "test_user", testQuestionBank)
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}
}

// ─────────────────────────────────────────────
// 3. StartSession — duplicate session ID is rejected
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_StartSession_Duplicate(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()
	id := uniqueID(t)

	if err := runner.StartSession(context.Background(), id, "test_user", testQuestionBank); err != nil {
		t.Fatalf("first StartSession: %v", err)
	}
	if err := runner.StartSession(context.Background(), id, "test_user", testQuestionBank); err == nil {
		t.Fatal("expected error for duplicate session ID, got nil")
	}
}

// ─────────────────────────────────────────────
// 4. SendTurn — unknown session ID is rejected
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_SendTurn_UnknownSession(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()

	var gotErr error
	for _, err := range runner.SendTurn(context.Background(), "no-such-session", "user_id", "anything") {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected error for unknown session ID, got nil")
	}
}

// ─────────────────────────────────────────────
// 5. SendTurn — recruiter question → Q: signal
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_SendTurn_QuestionSignal(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()
	id := uniqueID(t)

	if err := runner.StartSession(context.Background(), id, "test_user", testQuestionBank); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	transcript := "Recruiter: Can you walk me through how the Python GIL works and its impact on multi-threaded code?"
	signal := collectTurn(t, runner, id, transcript)

	t.Logf("signal = %q", signal)

	if !strings.HasPrefix(signal, "Q:") {
		t.Errorf("expected Q: signal, got %q", signal)
	}
}

// ─────────────────────────────────────────────
// 6. SendTurn — candidate answer → A: signal
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_SendTurn_AnswerSignal(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()
	id := uniqueID(t)

	if err := runner.StartSession(context.Background(), id, "test_user", testQuestionBank); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	transcript := "Candidate: The GIL is a mutex in CPython that allows only one thread to execute bytecode at a time, which prevents true parallelism in CPU-bound threads. I'd use multiprocessing for CPU-bound work instead."
	signal := collectTurn(t, runner, id, transcript)

	t.Logf("signal = %q", signal)

	if !strings.HasPrefix(signal, "A:") {
		t.Errorf("expected A: signal, got %q", signal)
	}
}

// ─────────────────────────────────────────────
// 7. SendTurn — filler speech → UNCLEAR
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_SendTurn_UnclearSignal(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()
	id := uniqueID(t)

	if err := runner.StartSession(context.Background(), id, "test_user", testQuestionBank); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	transcript := "Candidate: Uh... hmm... I mean..."
	signal := collectTurn(t, runner, id, transcript)

	t.Logf("signal = %q", signal)

	if !isValidSignal(signal) {
		t.Errorf("expected valid signal (Q:, A:, or UNCLEAR), got %q", signal)
	}
	if signal != "UNCLEAR" {
		t.Logf("note: expected UNCLEAR for filler speech, model returned %q", signal)
	}
}

// ─────────────────────────────────────────────
// 8. Multi-turn — question then answer in same session
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_MultiTurn(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()
	id := uniqueID(t)

	if err := runner.StartSession(context.Background(), id, "test_user", testQuestionBank); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	turn1 := collectTurn(t, runner, id,
		"Recruiter: Can you explain how the Python GIL affects multi-threaded applications?")
	t.Logf("turn 1 = %q", turn1)
	if !strings.HasPrefix(turn1, "Q:") {
		t.Errorf("turn 1: expected Q: signal, got %q", turn1)
	}

	turn2 := collectTurn(t, runner, id,
		"Candidate: The GIL prevents true parallelism in CPython because only one thread holds the interpreter lock at a time. For CPU-bound work I use multiprocessing instead.")
	t.Logf("turn 2 = %q", turn2)
	if !strings.HasPrefix(turn2, "A:") {
		t.Errorf("turn 2: expected A: signal, got %q", turn2)
	}
}

// ─────────────────────────────────────────────
// 9. Two concurrent sessions are independent
// ─────────────────────────────────────────────

func TestSignalingAgentRunner_TwoSessions(t *testing.T) {
	svc := newService(t)
	runner := svc.SignalingAgent()

	idA := uniqueID(t) + "_A"
	idB := uniqueID(t) + "_B"

	if err := runner.StartSession(context.Background(), idA, "test_user", testQuestionBank); err != nil {
		t.Fatalf("StartSession A: %v", err)
	}
	if err := runner.StartSession(context.Background(), idB, "test_user", testQuestionBank); err != nil {
		t.Fatalf("StartSession B: %v", err)
	}

	sigA := collectTurn(t, runner, idA,
		"Recruiter: Design a real-time leaderboard using Redis sorted sets.")
	sigB := collectTurn(t, runner, idB,
		"Candidate: I would use a Redis sorted set where the score is the player's point total.")

	t.Logf("session A = %q", sigA)
	t.Logf("session B = %q", sigB)

	if !isValidSignal(sigA) {
		t.Errorf("session A: invalid signal %q", sigA)
	}
	if !isValidSignal(sigB) {
		t.Errorf("session B: invalid signal %q", sigB)
	}
}
