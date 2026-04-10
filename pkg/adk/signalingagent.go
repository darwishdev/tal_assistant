package adk

import (
	"context"
	"iter"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// ─────────────────────────────────────────────
// signalingAgentRunner  (implements SignalingAgentRunner)
// ─────────────────────────────────────────────

type SignalingAgentState struct {
	QuestionBank []string
}

const (
	agentName         = "signaling_agent"
	agentDescription  = "sginaling agent"
	agentInstructions = `You are a real-time interview transcription signal extractor.

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
)

func NewSignalingAgentConfig(model model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:       model,
		Name:        agentName,
		Description: agentDescription,
		Instruction: agentInstructions,
	}
}
func (s *ADKService) NewSignalingAgentState(state SignalingAgentState) map[string]any {
	return map[string]any{
		"question_bank": state,
	}
}
func (s *ADKService) SignalingAgentSend(
	ctx context.Context,
	sessionID string,
	userID string,
	textPrompt string,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		events := s.signalingAgentRunner.Run(
			ctx,
			userID,
			sessionID,
			&genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: textPrompt}},
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
			text := s.extractText(event)
			if text == "" {
				continue
			}
			if !yield(text, nil) {
				return
			}
		}
	}
}
