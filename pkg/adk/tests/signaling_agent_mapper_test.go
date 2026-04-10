package tests

import (
	"context"
	"tal_assistant/pkg/adk/signalingagentmapper"
	"tal_assistant/pkg/adkutils"
	"testing"
)

var testMapperQuestionBank = signalingagentmapper.SignalingAgentMapperState{
	Questions: []signalingagentmapper.QuestionItem{
		{QuestionID: "q1", QuestionText: "Explain the Python GIL and its impact on multi-threaded programs."},
		{QuestionID: "q2", QuestionText: "Design a real-time leaderboard for a multiplayer game."},
	},
}

func TestSignalingAgentMapperRun(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	mapperState := svc.NewSignalingAgentMapperState(testMapperQuestionBank)
	if err := svc.SessionUpsert(ctx, id, testUserName, mapperState); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	cases := []struct {
		signal             string
		expectedQuestionID string
	}{
		{
			signal:             "Q:Explain the Python GIL and its impact on multi-threaded programs.",
			expectedQuestionID: "q1",
		},
		{
			signal:             "A:The GIL is a mutex in CPython that allows only one thread to execute bytecode at a time, preventing true parallelism in CPU-bound threads.",
			expectedQuestionID: "q1",
		},
		{
			signal:             "Q:Design a real-time leaderboard for a multiplayer game.",
			expectedQuestionID: "q2",
		},
	}

	for _, tc := range cases {
		req := adkutils.AgentRunRequest{
			Ctx:       ctx,
			SessionID: id,
			UserID:    testUserName,
			Prompt:    tc.signal,
		}
		questionID, err := svc.SignalingAgentMapperRun(req)
		if err != nil {
			t.Errorf("SignalingAgentMapperRun(%q): %v", tc.signal, err)
			continue
		}
		writeRecord(t, id, tc.signal, questionID)
		if questionID != tc.expectedQuestionID && questionID != "UNKNOWN" {
			t.Errorf("signal %q: got %q, want %q", tc.signal, questionID, tc.expectedQuestionID)
		}
		if questionID == "UNKNOWN" {
			t.Logf("signal %q: agent returned UNKNOWN (acceptable but not ideal)", tc.signal)
		}
	}
}
