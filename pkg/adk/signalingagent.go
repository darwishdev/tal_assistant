package adk

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"sync"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// ─────────────────────────────────────────────
// signalingAgentRunner  (implements SignalingAgentRunner)
// ─────────────────────────────────────────────

const signalingInstruction = `You are a real-time interview transcription signal extractor.

You will receive the live question bank for this interview session:
{question_bank}

Your ONLY job is to monitor the incoming transcript and emit ONE signal per
conversational turn. A signal must be EXACTLY one of the two formats below —
no extra text, no JSON, no punctuation outside the format:

  Q:<verbatim question text lifted from the transcript>
  A:<verbatim answer text lifted from the transcript>

Rules:
1. Emit Q: when the recruiter finishes asking a question.
2. Emit A: when the candidate finishes answering (turn is complete).
3. Do NOT emit anything for filler speech, pauses, or incomplete sentences.
4. Do NOT paraphrase. Copy the exact spoken text after the prefix.
5. One signal per response — never combine Q and A in the same turn.
6. If the turn is unclear or incomplete, output only: UNCLEAR`

const (
	signalingAppName = "interview_assistant"
)

type signalingAgentRunner struct {
	agent          agent.Agent
	sessionService session.Service
	runner         *runner.Runner

	// sessions maps sessionID → registered (prevents duplicate StartSession).
	mu       sync.RWMutex
	sessions map[string]struct{}
}

func newSignalingAgentRunner(llm model.LLM, svc session.Service) (*signalingAgentRunner, error) {
	a, err := llmagent.New(llmagent.Config{
		Name:        "signaling_agent",
		Model:       llm,
		Description: "Extracts Q:/A: signals from live interview transcriptions.",
		Instruction: signalingInstruction,
	})
	if err != nil {
		return nil, err
	}

	r, err := runner.New(runner.Config{
		AppName:        signalingAppName,
		Agent:          a,
		SessionService: svc,
	})
	if err != nil {
		return nil, err
	}

	return &signalingAgentRunner{
		agent:          a,
		sessionService: svc,
		runner:         r,
		sessions:       make(map[string]struct{}),
	}, nil
}

// StartSession implements SignalingAgentRunner.
func (sr *signalingAgentRunner) StartSession(
	ctx context.Context,
	sessionID, userID, questionBankJSON string,
) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if _, exists := sr.sessions[sessionID]; exists {
		return fmt.Errorf("signaling: session %q already started", sessionID)
	}

	_, err := sr.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   signalingAppName,
		UserID:    userID,
		SessionID: sessionID,
		State:     map[string]any{"question_bank": questionBankJSON},
	})
	if err != nil {
		return fmt.Errorf("signaling: create session: %w", err)
	}

	sr.sessions[sessionID] = struct{}{}
	return nil
}

// SendTurn implements SignalingAgentRunner.
func (sr *signalingAgentRunner) SendTurn(
	ctx context.Context,
	sessionID, userID, transcript string,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		sr.mu.RLock()
		_, exists := sr.sessions[sessionID]
		sr.mu.RUnlock()

		if !exists {
			yield("", fmt.Errorf("signaling: session %q not started", sessionID))
			return
		}

		events := sr.runner.Run(
			ctx,
			userID,
			sessionID,
			&genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: transcript}},
			},
			agent.RunConfig{
				StreamingMode: agent.StreamingModeSSE,
			},
		)

		for event, err := range events {
			if err != nil {
				yield("", err)
				return
			}
			text := extractText(event)
			if text == "" {
				continue
			}
			if !yield(text, nil) {
				return
			}
		}
	}
}
func extractText(ev *session.Event) string {
	if ev == nil || ev.LLMResponse.Content == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range ev.LLMResponse.Content.Parts {
		if part != nil && part.Text != "" {
			sb.WriteString(part.Text)
		}
	}
	return sb.String()
}
