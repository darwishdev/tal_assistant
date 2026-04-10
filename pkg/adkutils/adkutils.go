package adkutils

import (
	"context"
	"strings"

	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
)

type AgentRunRequest struct {
	Ctx       context.Context
	SessionID string
	UserID    string
	Prompt    any
}

func SessionEventToString(ev *session.Event) string {
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

func NewAgentRunner(
	ctx context.Context,
	appName string,
	sessionService session.Service,
	agentConfig llmagent.Config,
) (*runner.Runner, error) {
	a, err := llmagent.New(agentConfig)
	if err != nil {
		return nil, err
	}
	r, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             a,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}
