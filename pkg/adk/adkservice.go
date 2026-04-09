package adk

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"iter"
)

type SignalingAgentRunner interface {
	StartSession(ctx context.Context, sessionID string, userID string, questionBankJSON string) error
	SendTurn(ctx context.Context, sessionID string, userID string, transcript string) iter.Seq2[string, error]
}

type InterviewService interface {
	SignalingAgent() SignalingAgentRunner
}

// ADKService is the concrete implementation of InterviewService.
// Construct it with NewADKService; use it only through the InterviewService
// interface in the rest of the application.
type ADKService struct {
	llm            model.LLM
	sessionService session.Service
	signaling      *signalingAgentRunner
}

// NewADKService builds the service and wires up all agents.
// Pass any model.LLM — Gemini, a LiteLLM proxy, etc.
func NewADKService(llm model.LLM) (*ADKService, error) {
	svc := &ADKService{
		llm:            llm,
		sessionService: session.InMemoryService(),
	}
	sr, err := newSignalingAgentRunner(llm, svc.sessionService)
	if err != nil {
		return nil, fmt.Errorf("adkservice: init signaling agent: %w", err)
	}
	svc.signaling = sr
	return svc, nil
}

// SignalingAgent returns the SignalingAgentRunner.
// Satisfies InterviewService.
func (s *ADKService) SignalingAgent() SignalingAgentRunner {
	return s.signaling
}
