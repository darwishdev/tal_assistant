package adk

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"tal_assistant/pkg/adk/signalingagent"
	"tal_assistant/pkg/adkutils"
	"testing"
	"time"
)

// ─────────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────────

var (
	testUserName     = "test_user"
	testQuestionBank = signalingagent.SignalingAgentState{
		QuestionBank: []string{
			"Explain the Python GIL and its impact on multi-threaded programs.",
			"Design a real-time leaderboard for a multiplayer game.",
		},
	}
)

// newService builds an InterviewService backed by a real Gemini model.
// Skips the test if GOOGLE_API_KEY is not set.
func newService(t *testing.T) ADKServiceInterface {
	t.Helper()
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		t.Skip("GOOGLE_API_KEY not set — skipping integration test")
	}
	ctx := context.Background()
	svc, err := NewADKService(ctx, key)
	if err != nil {
		t.Fatalf("NewADKService: %v", err)
	}
	return svc
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
// 5. SendTurn — recruiter question → Q: signal
// ─────────────────────────────────────────────

// collectTurn calls SendTurn via the interface and collects the full signal.
// It also writes a structured record of input → output to testdata/<name>.txt.
func BuildSignalString(
	t *testing.T,
	signalSeq iter.Seq2[string, error],
) (string, error) {
	t.Helper()

	var sb strings.Builder
	for chunk, err := range signalSeq {
		if err != nil {
			return "", err
		}
		sb.WriteString(chunk)
	}
	signal := strings.TrimSpace(sb.String())
	return signal, nil
}
func TestSignalingAgentSend(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)
	agetnState := svc.NewSignalingAgentState(testQuestionBank)
	err := svc.SessionUpsert(ctx, id, testUserName, agetnState)
	if err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}
	inputs := []string{
		"Recruiter: Can you walk me through how the Python GIL works and its impact on multi-threaded code?",
		"Candidate: The GIL is a mutex in CPython that allows only one thread to execute bytecode at a time, which prevents true parallelism in CPU-bound threads. I'd use multiprocessing for CPU-bound work instead.",
	}
	for _, input := range inputs {
		req := adkutils.AgentRunRequest{
			Ctx:       ctx,
			SessionID: id,
			UserID:    testUserName,
			Prompt:    input,
		}
		signalChunks := svc.SignalingAgentRun(req)
		output, err := BuildSignalString(t, signalChunks)
		if err != nil {
			t.Logf("error proccessing signal :%s : %s", input, err.Error())
			t.Errorf("error from agent")
		}
		writeRecord(t, id, input, output)
	}
}
