package tests

import (
	"context"
	"encoding/json"
	"strings"
	"tal_assistant/pkg/adk/judgingagent"
	"tal_assistant/pkg/adkutils"
	"testing"
)

func TestJudgingAgentRun(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	// Interview context similar to what would be built from InterviewFindResult
	interviewContext := `# Interview Context

## Candidate
- Name: John Doe
- Email: john.doe@example.com
- Designation: Senior Software Engineer
- Skills: Python, Go, System Design, Microservices, Redis, WebSocket

## Job Position
- Title: Senior Backend Engineer
- Department: Engineering
- Location: Remote

## Interview Round
- Type: Technical
- Round: System Design Round
- Expected Average Rating: 7.50

## Expected Skills
- System Design: Ability to design scalable distributed systems
- Real-time Processing: Experience with streaming and event-driven architectures
- Database Design: Redis, PostgreSQL, caching strategies
`

	state := svc.NewJudgingAgentState(judgingagent.JudgingAgentState{
		InterviewContext: interviewContext,
	})
	if err := svc.SessionUpsert(ctx, id, testUserName, state); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	cases := []struct {
		name            string
		question        string
		answer          string
		expectPass      bool
		minScore        int
		maxScore        int
	}{
		{
			name:       "excellent_answer_high_score",
			question:   "Design a real-time interview assistant system that processes live audio, extracts Q&A pairs, and surfaces tips to the recruiter with under 2 seconds of latency. Walk me through the architecture.",
			answer:     "I'd use a WebSocket connection for real-time audio streaming from the client. Audio chunks go through a VAD (Voice Activity Detection) module to detect speech boundaries, then get sent to a streaming transcription service like Deepgram or AssemblyAI. Transcripts are pushed onto a Redis message queue. A consumer picks them up, calls an LLM inference service for tip generation — we'd optimize LLM calls to stay within a 500ms latency budget using techniques like batching and caching. The results are pushed back over WebSocket to the recruiter UI. The whole pipeline is event-driven to keep end-to-end latency under 2 seconds. For reliability, I'd implement circuit breakers and fallback mechanisms.",
			expectPass: true,
			minScore:   80,
			maxScore:   100,
		},
		{
			name:       "good_answer_passes_threshold",
			question:   "Explain the Python GIL and its impact on multi-threaded programs.",
			answer:     "The GIL (Global Interpreter Lock) is a mutex in CPython that prevents multiple threads from executing Python bytecode simultaneously. This means that even on multi-core systems, only one thread can execute Python code at a time. For CPU-bound tasks, this severely limits performance. However, I/O-bound operations release the GIL, so multi-threading can still be beneficial there. For true parallelism, you need to use multiprocessing instead.",
			expectPass: true,
			minScore:   70,
			maxScore:   90,
		},
		{
			name:       "weak_answer_fails_threshold",
			question:   "Design a real-time interview assistant system that processes live audio, extracts Q&A pairs, and surfaces tips to the recruiter with under 2 seconds of latency. Walk me through the architecture.",
			answer:     "I would use a microphone to capture audio and send it to a server. The server would process it and send results back to the UI.",
			expectPass: false,
			minScore:   0,
			maxScore:   50,
		},
		{
			name:       "partial_answer_borderline_score",
			question:   "Explain the Python GIL and its impact on multi-threaded programs.",
			answer:     "The GIL is something in Python that affects threading. It makes multi-threaded programs slower because only one thread can run at a time.",
			expectPass: false,
			minScore:   10,
			maxScore:   40,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Stage 1: Send question (no response expected)
			questionPrompt := "Q: " + tc.question
			for _, err := range svc.JudgingAgentRun(adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    questionPrompt,
			}) {
				if err != nil {
					t.Fatalf("JudgingAgentRun (question stage): %v", err)
				}
				// Consume and discard any response (agent should not respond to questions)
			}
			t.Logf("Question sent to judging agent (no response expected)")

			// Stage 2: Send answer and collect JSON judgment
			answerPrompt := "A: " + tc.answer
			var sb strings.Builder
			for chunk, err := range svc.JudgingAgentRun(adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    answerPrompt,
			}) {
				if err != nil {
					t.Fatalf("JudgingAgentRun (answer stage): %v", err)
				}
				// Handle deduplication: ADK SDK sends incremental chunks then final full chunk
				if sb.Len() > 0 && chunk == sb.String() {
					t.Logf("Suppressed duplicate final chunk (len=%d)", len(chunk))
					break
				}
				sb.WriteString(chunk)
			}

			output := strings.TrimSpace(sb.String())
			if output == "" {
				t.Fatalf("JudgingAgentRun returned empty output")
			}

			t.Logf("Raw output:\n%s", output)

			// Parse JSON judgment
			var judgment struct {
				Score           int      `json:"score"`
				Pass            bool     `json:"pass"`
				Strengths       []string `json:"strengths"`
				Weaknesses      []string `json:"weaknesses"`
				MissingKeywords []string `json:"missing_keywords"`
				Verdict         string   `json:"verdict"`
			}

			if err := json.Unmarshal([]byte(output), &judgment); err != nil {
				t.Fatalf("Failed to parse judgment JSON: %v\nOutput: %s", err, output)
			}

			// Verify judgment fields
			if judgment.Score < tc.minScore || judgment.Score > tc.maxScore {
				t.Errorf("Score %d out of expected range [%d, %d]", judgment.Score, tc.minScore, tc.maxScore)
			}

			if judgment.Pass != tc.expectPass {
				t.Errorf("Expected pass=%v, got pass=%v", tc.expectPass, judgment.Pass)
			}

			if judgment.Verdict == "" {
				t.Errorf("Verdict should not be empty")
			}

			if tc.expectPass && len(judgment.Strengths) == 0 {
				t.Errorf("Passing answers should have at least one strength")
			}

			if !tc.expectPass && len(judgment.Weaknesses) == 0 {
				t.Errorf("Failing answers should have at least one weakness")
			}

			t.Logf("Judgment: score=%d pass=%v strengths=%d weaknesses=%d missing=%d",
				judgment.Score, judgment.Pass, len(judgment.Strengths), len(judgment.Weaknesses), len(judgment.MissingKeywords))
			t.Logf("Verdict: %s", judgment.Verdict)

			// Write record for later inspection
			writeRecord(t, id, questionPrompt+"\n"+answerPrompt, output)
		})
	}
}

func TestJudgingAgentMultipleQAPairs(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	id := uniqueID(t)

	interviewContext := `# Interview Context

## Candidate
- Name: Jane Smith
- Email: jane.smith@example.com
- Designation: Software Engineer
- Skills: JavaScript, React, Node.js, MongoDB

## Job Position
- Title: Full Stack Developer
- Department: Product Engineering
- Location: New York

## Interview Round
- Type: Technical
- Round: Coding Round
- Expected Average Rating: 6.50

## Expected Skills
- JavaScript: Proficiency in ES6+ and async programming
- React: Component design and state management
- Node.js: REST API development and middleware
`

	state := svc.NewJudgingAgentState(judgingagent.JudgingAgentState{
		InterviewContext: interviewContext,
	})
	if err := svc.SessionUpsert(ctx, id, testUserName, state); err != nil {
		t.Fatalf("SessionUpsert: %v", err)
	}

	qaPairs := []struct {
		question string
		answer   string
	}{
		{
			question: "Explain the difference between let, const, and var in JavaScript.",
			answer:   "var is function-scoped and hoisted, while let and const are block-scoped. const creates a read-only reference, meaning you can't reassign it, but the value itself can be mutable if it's an object or array. let allows reassignment.",
		},
		{
			question: "What are React hooks and why were they introduced?",
			answer:   "Hooks let you use state and other React features in functional components. They were introduced to avoid the complexity of class components and make it easier to reuse stateful logic between components. Common hooks include useState, useEffect, and useContext.",
		},
	}

	for i, pair := range qaPairs {
		t.Run("qa_pair_"+string(rune('A'+i)), func(t *testing.T) {
			// Send question
			questionPrompt := "Q: " + pair.question
			for _, err := range svc.JudgingAgentRun(adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    questionPrompt,
			}) {
				if err != nil {
					t.Fatalf("JudgingAgentRun (question %d): %v", i+1, err)
				}
			}

			// Send answer and get judgment
			answerPrompt := "A: " + pair.answer
			var sb strings.Builder
			for chunk, err := range svc.JudgingAgentRun(adkutils.AgentRunRequest{
				Ctx:       ctx,
				SessionID: id,
				UserID:    testUserName,
				Prompt:    answerPrompt,
			}) {
				if err != nil {
					t.Fatalf("JudgingAgentRun (answer %d): %v", i+1, err)
				}
				if sb.Len() > 0 && chunk == sb.String() {
					break
				}
				sb.WriteString(chunk)
			}

			output := strings.TrimSpace(sb.String())
			var judgment map[string]interface{}
			if err := json.Unmarshal([]byte(output), &judgment); err != nil {
				t.Fatalf("Failed to parse judgment JSON for pair %d: %v", i+1, err)
			}

			t.Logf("Q&A pair %d judgment: %v", i+1, judgment)
			writeRecord(t, id, questionPrompt+"\n"+answerPrompt, output)
		})
	}
}
