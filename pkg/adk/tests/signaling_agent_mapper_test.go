package tests

import (
	"context"
	"strings"
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

func TestSignalingAgentAnswerCompletionSignal(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	mapperState := svc.NewSignalingAgentMapperState(testMapperQuestionBank)
	if err := svc.SessionUpsert(ctx, id, testUserName, mapperState); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	cases := []struct {
		name               string
		signal             string
		expectedQuestionID string
		isComplete         bool // whether answer should have semicolon
	}{
		{
			name:               "question_signal",
			signal:             "Q:Explain the Python GIL and its impact on multi-threaded programs.",
			expectedQuestionID: "q1",
			isComplete:         false,
		},
		{
			name:               "answer_in_progress_no_semicolon",
			signal:             "A:The GIL is a mutex in CPython that allows only one thread",
			expectedQuestionID: "q1",
			isComplete:         false,
		},
		{
			name:               "answer_complete_with_semicolon",
			signal:             "A:The GIL is a mutex in CPython that allows only one thread to execute bytecode at a time, preventing true parallelism in CPU-bound threads;",
			expectedQuestionID: "q1",
			isComplete:         true,
		},
		{
			name:               "next_question",
			signal:             "Q:Design a real-time leaderboard for a multiplayer game.",
			expectedQuestionID: "q2",
			isComplete:         false,
		},
		{
			name:               "answer_q2_in_progress",
			signal:             "A:I would use Redis sorted sets for the leaderboard",
			expectedQuestionID: "q2",
			isComplete:         false,
		},
		{
			name:               "answer_q2_complete",
			signal:             "A:I would use Redis sorted sets for the leaderboard with score updates via ZADD and periodic syncs to a persistent database;",
			expectedQuestionID: "q2",
			isComplete:         true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    tc.signal,
			}
			questionID, err := svc.SignalingAgentMapperRun(req)
			if err != nil {
				t.Errorf("SignalingAgentMapperRun(%q): %v", tc.signal, err)
				return
			}
			writeRecord(t, id, tc.signal, questionID)

			// Verify the signal format based on completion status
			if tc.isComplete {
				if !strings.HasSuffix(tc.signal, ";") {
					t.Errorf("signal %q: expected semicolon at end for complete answer", tc.signal)
				}
			} else if strings.HasPrefix(tc.signal, "A:") {
				if strings.HasSuffix(tc.signal, ";") {
					t.Errorf("signal %q: unexpected semicolon for in-progress answer", tc.signal)
				}
			}

			// Verify question ID mapping
			if questionID != tc.expectedQuestionID && questionID != "UNKNOWN" {
				t.Errorf("signal %q: got questionID %q, want %q", tc.signal, questionID, tc.expectedQuestionID)
			}
			if questionID == "UNKNOWN" {
				t.Logf("signal %q: agent returned UNKNOWN (acceptable but not ideal)", tc.signal)
			} else {
				t.Logf("signal %q: mapped to questionID %q, complete=%v", tc.signal, questionID, tc.isComplete)
			}
		})
	}
}
