package signalingagentmapper

import (
	"strings"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

type QuestionItem struct {
	QuestionID   string
	QuestionText string
}

type SignalingAgentMapperState struct {
	Questions []QuestionItem
}

type SignalingAgentMapper struct {
	agentName string
	llm       *model.LLM
}

func NewSignalingAgentMapper(llm *model.LLM) *SignalingAgentMapper {
	return &SignalingAgentMapper{
		agentName: agentName,
		llm:       llm,
	}
}

func (a *SignalingAgentMapper) NewAgentConfig(model model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:       model,
		Name:        agentName,
		Description: agentDescription,
		Instruction: agentInstructions,
	}
}

func (a *SignalingAgentMapper) NewAgentState(state SignalingAgentMapperState) map[string]any {
	return map[string]any{
		"questions": state.Questions,
	}
}

func (a *SignalingAgentMapper) Run(
	r *runner.Runner,
	req adkutils.AgentRunRequest,
) (string, error) {
	events := r.Run(
		req.Ctx,
		req.UserID,
		req.SessionID,
		&genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: req.Prompt.(string)}},
		},
		agent.RunConfig{
			StreamingMode: agent.StreamingModeNone,
		},
	)

	var sb strings.Builder
	for event, err := range events {
		if err != nil {
			return "", err
		}
		text := adkutils.SessionEventToString(event)
		if text != "" {
			sb.WriteString(text)
		}
	}
	return strings.TrimSpace(sb.String()), nil
}
