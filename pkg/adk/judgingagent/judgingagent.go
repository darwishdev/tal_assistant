package judgingagent

import (
	"fmt"
	"iter"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

type JudgingAgentState struct {
	InterviewContext string
}

type JudgingAgent struct {
	agentName string
	llm       *model.LLM
}

func NewJudgingAgent(llm *model.LLM) *JudgingAgent {
	return &JudgingAgent{
		agentName: agentName,
		llm:       llm,
	}
}

func (a *JudgingAgent) NewAgentConfig(m model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:       m,
		Name:        agentName,
		Description: agentDescription,
		Instruction: agentInstructions,
	}
}

func (a *JudgingAgent) NewAgentState(state JudgingAgentState) map[string]any {
	return map[string]any{
		"interview_context": state.InterviewContext,
	}
}

func (a *JudgingAgent) Run(
	r *runner.Runner,
	req adkutils.AgentRunRequest,
) iter.Seq2[string, error] {
	// Prompt is a plain string (either question or answer)
	text, ok := req.Prompt.(string)
	if !ok {
		return func(yield func(string, error) bool) {
			yield("", fmt.Errorf("invalid prompt type: expected string, got %T", req.Prompt))
		}
	}

	return func(yield func(string, error) bool) {
		events := r.Run(
			req.Ctx,
			req.UserID,
			req.SessionID,
			&genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: text}},
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
			chunk := adkutils.SessionEventToString(event)
			if chunk == "" {
				continue
			}
			if !yield(chunk, nil) {
				return
			}
		}
	}
}
