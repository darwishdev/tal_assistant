package adk

import (
	"context"
	"fmt"
	"iter"
	"tal_assistant/pkg/adk/signalingagent"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
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
	NewSignalingAgentState(req signalingagent.SignalingAgentState) map[string]any
	SignalingAgentRun(req adkutils.AgentRunRequest) iter.Seq2[string, error]
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
	singalinAgent        *signalingagent.SignalingAgent
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
	// signaling agent
	singalinAgent := signalingagent.NewSignalingAgent(&geminiModel)
	signalingAgentConfig := singalinAgent.NewAgentConfig(geminiModel)
	sginalingAgentRunner, err := NewAgentRunner(
		ctx,
		appName,
		sessionService,
		*signalingAgentConfig,
	)

	return &ADKService{
		sessionService:       sessionService,
		geminiLiteModel:      geminiLiteModel,
		singalinAgent:        singalinAgent,
		appName:              appName,
		geminiModel:          geminiModel,
		geminiProModel:       geminiProModel,
		signalingAgentRunner: sginalingAgentRunner,
	}, nil
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
