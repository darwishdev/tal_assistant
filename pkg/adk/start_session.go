package adk

import (
	"context"
	"fmt"
	"tal_assistant/pkg/adk/judgingagent"
	"tal_assistant/pkg/adk/nextquestionextender"
	"tal_assistant/pkg/adk/nextquestionindicator"
	"tal_assistant/pkg/adk/signalingagent"
	"tal_assistant/pkg/adk/signalingagentmapper"
	"tal_assistant/pkg/adkutils"
	"time"
)

type InterviewSessions struct {
	SignalingAgentSessionID        string
	SignalingAgentMapperSessionID  string
	NextQuestionIndicatorSessionID string
	NextQuestionExtenderSessionID  string
	JudgingAgentSessionID          string
}

func (s *ADKService) StartSession(
	ctx context.Context,
	userID string,
	questionBank []adkutils.QuestionBankQuestion,
) (InterviewSessions, error) {
	base := fmt.Sprintf("%s_%d", userID, time.Now().UnixMilli())
	sessions := InterviewSessions{
		SignalingAgentSessionID:        base + "_signaling",
		SignalingAgentMapperSessionID:  base + "_mapper",
		NextQuestionIndicatorSessionID: base + "_indicator",
		NextQuestionExtenderSessionID:  base + "_extender",
		JudgingAgentSessionID:          base + "_judging",
	}

	// signaling agent — needs question texts only
	questionTexts := make([]string, len(questionBank))
	for i, q := range questionBank {
		questionTexts[i] = q.Question
	}
	signalingState := s.singalinAgent.NewAgentState(signalingagent.SignalingAgentState{
		QuestionBank: questionTexts,
	})
	if err := s.SessionUpsert(ctx, sessions.SignalingAgentSessionID, userID, signalingState); err != nil {
		return InterviewSessions{}, fmt.Errorf("start session: signaling agent: %w", err)
	}

	// signaling agent mapper — needs question ID + text pairs
	questionItems := make([]signalingagentmapper.QuestionItem, len(questionBank))
	for i, q := range questionBank {
		questionItems[i] = signalingagentmapper.QuestionItem{
			QuestionID:   q.ID,
			QuestionText: q.Question,
		}
	}
	mapperState := s.signalingAgentMapper.NewAgentState(signalingagentmapper.SignalingAgentMapperState{
		Questions: questionItems,
	})
	if err := s.SessionUpsert(ctx, sessions.SignalingAgentMapperSessionID, userID, mapperState); err != nil {
		return InterviewSessions{}, fmt.Errorf("start session: signaling agent mapper: %w", err)
	}

	// next question indicator — needs full question bank
	indicatorState := s.nextQuestionIndicator.NewAgentState(nextquestionindicator.NextQuestionIndicatorState{
		QuestionBank: questionBank,
	})
	if err := s.SessionUpsert(ctx, sessions.NextQuestionIndicatorSessionID, userID, indicatorState); err != nil {
		return InterviewSessions{}, fmt.Errorf("start session: next question indicator: %w", err)
	}

	// next question extender — needs full question bank
	extenderState := s.nextQuestionExtender.NewAgentState(nextquestionextender.NextQuestionExtenderState{
		QuestionBank: questionBank,
	})
	if err := s.SessionUpsert(ctx, sessions.NextQuestionExtenderSessionID, userID, extenderState); err != nil {
		return InterviewSessions{}, fmt.Errorf("start session: next question extender: %w", err)
	}

	// judging agent — interview context will be set when judging starts
	judgingState := s.judgingAgent.NewAgentState(judgingagent.JudgingAgentState{
		InterviewContext: "", // Set later when interview details are available
	})
	if err := s.SessionUpsert(ctx, sessions.JudgingAgentSessionID, userID, judgingState); err != nil {
		return InterviewSessions{}, fmt.Errorf("start session: judging agent: %w", err)
	}

	return sessions, nil
}

// SetJudgingAgentContext updates the interview context for the judging agent after session initialization.
// This should be called once interview details are available.
func (s *ADKService) SetJudgingAgentContext(
	ctx context.Context,
	sessionID string,
	userID string,
	interviewContext string,
) error {
	judgingState := s.judgingAgent.NewAgentState(judgingagent.JudgingAgentState{
		InterviewContext: interviewContext,
	})
	return s.SessionUpsert(ctx, sessionID, userID, judgingState)
}
