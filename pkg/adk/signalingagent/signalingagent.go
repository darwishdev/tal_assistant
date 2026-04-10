package signalingagent

import (
	"iter"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

// ─────────────────────────────────────────────
// signalingAgentRunner  (implements SignalingAgentRunner)
// ─────────────────────────────────────────────

type SignalingAgentState struct {
	QuestionBank []string
}
type SignalingAgent struct {
	agentName string
	llm       *model.LLM
}

func NewSignalingAgent(llm *model.LLM) *SignalingAgent {
	return &SignalingAgent{
		agentName: agentName,
		llm:       llm,
	}
}
func (a *SignalingAgent) NewAgentConfig(model model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:       model,
		Name:        agentName,
		Description: agentDescription,
		Instruction: agentInstructions,
	}
}
func (a *SignalingAgent) NewAgentState(state SignalingAgentState) map[string]any {
	return map[string]any{
		"question_bank": state,
	}
}
func (a *SignalingAgent) Run(
	runner *runner.Runner,
	req adkutils.AgentRunRequest,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		events := runner.Run(
			req.Ctx,
			req.UserID,
			req.SessionID,
			&genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: req.Prompt.(string)}},
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
			text := adkutils.SessionEventToString(event)
			if text == "" {
				continue
			}
			if !yield(text, nil) {
				return
			}
		}
	}
}
