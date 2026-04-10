package tests

import (
	"context"
	"strings"
	"tal_assistant/pkg/adk/signalingagent"
	"tal_assistant/pkg/adkutils"
	"testing"
)

var testQuestionBank = signalingagent.SignalingAgentState{
	QuestionBank: []string{
		"Explain the Python GIL and its impact on multi-threaded programs.",
		"Design a real-time leaderboard for a multiplayer game.",
	},
}

func TestSignalingAgentSend(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	agentState := svc.NewSignalingAgentState(testQuestionBank)
	if err := svc.SessionUpsert(ctx, id, testUserName, agentState); err != nil {
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
		var sb strings.Builder
		for chunk, err := range svc.SignalingAgentRun(req) {
			if err != nil {
				t.Errorf("error processing signal %q: %v", input, err)
				break
			}
			sb.WriteString(chunk)
		}
		output := strings.TrimSpace(sb.String())
		writeRecord(t, id, input, output)
	}
}
