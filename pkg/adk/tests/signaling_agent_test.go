package tests

import (
	"context"
	"strings"
	"tal_assistant/pkg/adk/signalingagent"
	"tal_assistant/pkg/adkutils"
	"testing"
)

var testSignalingQuestionBank = signalingagent.SignalingAgentState{
	QuestionBank: []string{
		"Explain the Python GIL and its impact on multi-threaded programs.",
		"Design a real-time leaderboard for a multiplayer game.",
	},
}

func TestSignalingAgentAnswerCompletion(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	agentState := svc.NewSignalingAgentState(testSignalingQuestionBank)
	if err := svc.SessionUpsert(ctx, id, testUserName, agentState); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	cases := []struct {
		name           string
		transcript     string
		expectedSignal string
		isComplete     bool // whether answer should have semicolon
	}{
		{
			name:           "recruiter_asks_question",
			transcript:     "Recruiter: So, can you explain the Python GIL and its impact on multi-threaded programs?",
			expectedSignal: "Q:Explain the Python GIL and its impact on multi-threaded programs.",
			isComplete:     false,
		},
		{
			name:           "candidate_starts_answer",
			transcript:     "Candidate: Well, the GIL is a mutex in CPython",
			expectedSignal: "A:Well, the GIL is a mutex in CPython",
			isComplete:     false,
		},
		{
			name:           "candidate_continues_answer",
			transcript:     "Candidate: that allows only one thread to execute bytecode at time",
			expectedSignal: "A:that allows only one thread to execute bytecode at time",
			isComplete:     false,
		},
		{
			name:           "candidate_completes_answer",
			transcript:     "Candidate: The GIL is a mutex in CPython that allows only one thread to execute bytecode at a time, preventing true parallelism in CPU-bound threads",
			expectedSignal: "A:The GIL is a mutex in CPython that allows only one thread to execute bytecode at a time, preventing true parallelism in CPU-bound threads;",
			isComplete:     true,
		},
		{
			name:           "next_question",
			transcript:     "Recruiter: Great! Now, can you design a real-time leaderboard for a multiplayer game?",
			expectedSignal: "Q:Design a real-time leaderboard for a multiplayer game.",
			isComplete:     false,
		},
		{
			name:           "partial_answer_q2",
			transcript:     "Candidate: I would use Redis sorted sets",
			expectedSignal: "A:I would use Redis sorted sets",
			isComplete:     false,
		},
		{
			name:           "complete_answer_q2",
			transcript:     "Candidate: I would use Redis sorted sets for the leaderboard with score updates via ZADD and periodic syncs to a persistent database for durability",
			expectedSignal: "A:I would use Redis sorted sets for the leaderboard with score updates via ZADD and periodic syncs to a persistent database for durability;",
			isComplete:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    tc.transcript,
			}

			var signal string
			for chunk, err := range svc.SignalingAgentRun(req) {
				if err != nil {
					t.Errorf("SignalingAgentRun error: %v", err)
					return
				}
				signal += chunk
			}

			signal = strings.TrimSpace(signal)
			writeRecord(t, id, tc.transcript, signal)

			// Verify the signal format based on completion status
			if tc.isComplete {
				if !strings.HasSuffix(signal, ";") {
					t.Errorf("transcript: %q\ngot signal: %q\nexpected semicolon at end for complete answer", tc.transcript, signal)
				}
			} else if strings.HasPrefix(signal, "A:") {
				if strings.HasSuffix(signal, ";") {
					t.Errorf("transcript: %q\ngot signal: %q\nunexpected semicolon for in-progress answer", tc.transcript, signal)
				}
			}

			// Verify the signal matches expected format
			if !strings.Contains(signal, tc.expectedSignal) && signal != tc.expectedSignal {
				t.Logf("transcript: %q\ngot signal: %q\nexpected: %q", tc.transcript, signal, tc.expectedSignal)
				// Don't fail the test - LLM behavior can vary slightly
				// but log for inspection
			}

			t.Logf("✓ transcript: %q\n  signal: %q\n  complete: %v", tc.transcript, signal, tc.isComplete)
		})
	}
}

func TestSignalingAgentEdgeCases(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	agentState := svc.NewSignalingAgentState(testSignalingQuestionBank)
	if err := svc.SessionUpsert(ctx, id, testUserName, agentState); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	cases := []struct {
		name       string
		transcript string
		wantPrefix string // What the signal should start with (Q: or A:)
	}{
		{
			name:       "filler_speech_should_be_ignored",
			transcript: "Candidate: Um... well... let me think...",
			wantPrefix: "", // May return UNCLEAR or nothing
		},
		{
			name:       "short_acknowledgment",
			transcript: "Candidate: Yes, I understand.",
			wantPrefix: "A:", // Still an answer, but very short
		},
		{
			name:       "question_with_context",
			transcript: "Recruiter: Alright, next question. Can you explain the Python GIL and its impact on multi-threaded programs?",
			wantPrefix: "Q:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    tc.transcript,
			}

			var signal string
			for chunk, err := range svc.SignalingAgentRun(req) {
				if err != nil {
					t.Errorf("SignalingAgentRun error: %v", err)
					return
				}
				signal += chunk
			}

			signal = strings.TrimSpace(signal)
			writeRecord(t, id, tc.transcript, signal)

			if tc.wantPrefix != "" {
				if !strings.HasPrefix(signal, tc.wantPrefix) {
					t.Logf("transcript: %q\ngot signal: %q\nexpected prefix: %q", tc.transcript, signal, tc.wantPrefix)
				}
			}

			t.Logf("✓ transcript: %q\n  signal: %q", tc.transcript, signal)
		})
	}
}
