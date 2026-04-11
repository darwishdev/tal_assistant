package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"tal_assistant/pkg/adk/signalingagent"
	"tal_assistant/pkg/adk/signalingagentmapper"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/session"
)

// stateValueAs decodes a raw session-state value (which may be an original Go
// struct or a map[string]any after internal serialization) into T by going
// through a JSON round-trip.
func stateValueAs[T any](raw any) (T, error) {
	var result T
	data, err := json.Marshal(raw)
	if err != nil {
		return result, fmt.Errorf("marshal state value: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("unmarshal state value as %T: %w", result, err)
	}
	return result, nil
}

func (s *ADKService) getSession(ctx context.Context, userID, sessionID string) (session.Session, error) {
	resp, err := s.sessionService.Get(ctx, &session.GetRequest{
		AppName:   s.appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("get session %s: %w", sessionID, err)
	}
	return resp.Session, nil
}

func appendStateEvent(ctx context.Context, svc session.Service, sess session.Session, delta map[string]any) error {
	event := session.NewEvent("")
	event.Actions.StateDelta = delta
	return svc.AppendEvent(ctx, sess, event)
}

// AppendQuestionToSessions pushes the new question into the active sessions of
// the signaling agent, mapper, and NQI agent so that they are aware of it on
// their next invocation.
func (s *ADKService) AppendQuestionToSessions(
	ctx context.Context,
	userID string,
	signalingSessionID string,
	mapperSessionID string,
	indicatorSessionID string,
	question adkutils.QuestionBankQuestion,
) error {
	if err := s.appendToSignalingSession(ctx, userID, signalingSessionID, question); err != nil {
		return fmt.Errorf("signaling session: %w", err)
	}
	if err := s.appendToMapperSession(ctx, userID, mapperSessionID, question); err != nil {
		return fmt.Errorf("mapper session: %w", err)
	}
	if err := s.appendToIndicatorSession(ctx, userID, indicatorSessionID, question); err != nil {
		return fmt.Errorf("indicator session: %w", err)
	}
	return nil
}

func (s *ADKService) appendToSignalingSession(
	ctx context.Context,
	userID, sessionID string,
	question adkutils.QuestionBankQuestion,
) error {
	sess, err := s.getSession(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	raw, err := sess.State().Get("question_bank")
	if err != nil {
		return fmt.Errorf("read question_bank: %w", err)
	}
	state, err := stateValueAs[signalingagent.SignalingAgentState](raw)
	if err != nil {
		return err
	}
	state.QuestionBank = append(state.QuestionBank, question.Question)
	return appendStateEvent(ctx, s.sessionService, sess, map[string]any{
		"question_bank": state,
	})
}

func (s *ADKService) appendToMapperSession(
	ctx context.Context,
	userID, sessionID string,
	question adkutils.QuestionBankQuestion,
) error {
	sess, err := s.getSession(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	raw, err := sess.State().Get("questions")
	if err != nil {
		return fmt.Errorf("read questions: %w", err)
	}
	items, err := stateValueAs[[]signalingagentmapper.QuestionItem](raw)
	if err != nil {
		return err
	}
	items = append(items, signalingagentmapper.QuestionItem{
		QuestionID:   question.ID,
		QuestionText: question.Question,
	})
	return appendStateEvent(ctx, s.sessionService, sess, map[string]any{
		"questions": items,
	})
}

func (s *ADKService) appendToIndicatorSession(
	ctx context.Context,
	userID, sessionID string,
	question adkutils.QuestionBankQuestion,
) error {
	sess, err := s.getSession(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	raw, err := sess.State().Get("question_bank")
	if err != nil {
		return fmt.Errorf("read question_bank: %w", err)
	}
	bank, err := stateValueAs[[]adkutils.QuestionBankQuestion](raw)
	if err != nil {
		return err
	}
	bank = append(bank, question)
	return appendStateEvent(ctx, s.sessionService, sess, map[string]any{
		"question_bank": bank,
	})
}
