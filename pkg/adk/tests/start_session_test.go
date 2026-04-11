package tests

import (
	"context"
	"tal_assistant/pkg/adkutils"
	"testing"
)

func TestStartSession(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	questionBank := []adkutils.QuestionBankQuestion{testQuestion1, testQuestion2}

	sessions, err := svc.StartSession(ctx, testUserName, questionBank)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	if sessions.SignalingAgentSessionID == "" {
		t.Error("expected non-empty SignalingAgentSessionID")
	}
	if sessions.SignalingAgentMapperSessionID == "" {
		t.Error("expected non-empty SignalingAgentMapperSessionID")
	}
	if sessions.NextQuestionIndicatorSessionID == "" {
		t.Error("expected non-empty NextQuestionIndicatorSessionID")
	}
	if sessions.NextQuestionExtenderSessionID == "" {
		t.Error("expected non-empty NextQuestionExtenderSessionID")
	}

	t.Logf("sessions: signaling=%s mapper=%s indicator=%s extender=%s",
		sessions.SignalingAgentSessionID,
		sessions.SignalingAgentMapperSessionID,
		sessions.NextQuestionIndicatorSessionID,
		sessions.NextQuestionExtenderSessionID,
	)

	// calling again with the same user should be idempotent (SessionUpsert is a no-op if session exists)
	sessions2, err := svc.StartSession(ctx, testUserName, questionBank)
	if err != nil {
		t.Fatalf("StartSession (second call): %v", err)
	}
	if sessions2.SignalingAgentSessionID == sessions.SignalingAgentSessionID {
		t.Error("expected different session IDs on second call (timestamp-based)")
	}
}
