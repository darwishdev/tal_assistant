package adk

import (
	"context"
	"fmt"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
	"iter"
	"strings"
)

const (
	geminiFlashLiteModelName = "gemini-3.1-flash-lite-preview"
	geminiProModelName       = "gemini-3.1-pro-preview"
	geminiFlashModelName     = "gemini-3-flash-preview"
)

type ADKServiceInterface interface {
	SessionUpsert(
		ctx context.Context,
		sessionID string,
		userID string,
		state map[string]any,
	) error
	// sginaling agent
	NewSignalingAgentState(state SignalingAgentState) map[string]any
	SignalingAgentSend(
		ctx context.Context,
		sessionID string,
		userID string,
		textPrompt string,
	) iter.Seq2[string, error]
}

// ADKService is the concrete implementation of InterviewService.
// Construct it with NewADKService; use it only through the InterviewService
// interface in the rest of the application.
type ADKService struct {
	geminiLiteModel      model.LLM
	geminiProModel       model.LLM
	geminiModel          model.LLM
	appName              string
	sessionService       session.Service
	signalingAgentRunner *runner.Runner
}

// NewADKService builds the service and wires up all agents.
// Pass any model.LLM — Gemini, a LiteLLM proxy, etc.
func NewADKService(ctx context.Context, geminiApiKey string) (ADKServiceInterface, error) {
	appName := "interview_assistant"
	geminiLiteModel, err := gemini.NewModel(ctx, geminiFlashLiteModelName, &genai.ClientConfig{
		APIKey: geminiApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating models for : %s : %w", geminiFlashLiteModelName, err)
	}
	geminiModel, err := gemini.NewModel(ctx, geminiFlashModelName, &genai.ClientConfig{
		APIKey: geminiApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating models for : %s : %w", geminiFlashModelName, err)
	}
	geminiProModel, err := gemini.NewModel(ctx, geminiProModelName, &genai.ClientConfig{
		APIKey: geminiApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating models for : %s : %w", geminiProModelName, err)
	}

	sessionService := session.InMemoryService()
	signalingAgentConfig := NewSignalingAgentConfig(geminiModel)
	sginalingAgentRunner, err := newAgentRunner(
		ctx,
		appName,
		sessionService,
		*signalingAgentConfig,
	)
	svc := &ADKService{
		sessionService:       sessionService,
		geminiLiteModel:      geminiLiteModel,
		appName:              appName,
		geminiModel:          geminiModel,
		geminiProModel:       geminiProModel,
		signalingAgentRunner: sginalingAgentRunner,
	}
	return svc, nil
}
func (s *ADKService) SessionUpsert(
	ctx context.Context,
	sessionID string,
	userID string,
	state map[string]any,
) error {
	_, err := s.sessionService.Get(ctx, &session.GetRequest{
		AppName:   s.appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err == nil {
		return nil
	}
	_, err = s.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   s.appName,
		UserID:    userID,
		SessionID: sessionID,
		State:     state,
	})
	if err != nil {
		return fmt.Errorf("signaling: create session: %w", err)
	}
	return nil
}

func newAgentRunner(
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
func (s *ADKService) extractText(ev *session.Event) string {
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
