package repo

import (
	"context"
	"fmt"
	"tal_assistant/pkg/adkutils"
)

func (r *EventRepo) EventFind(ctx context.Context, eventID string) ([]byte, error) {
	e, err := r.db.FindEventData(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to find event data: %w", err)
	}
	return e, nil
}

func (r *EventRepo) EventCreate(ctx context.Context, eventID string, eventData []byte) error {
	err := r.db.SaveEventData(ctx, eventID, eventData)
	if err != nil {
		return fmt.Errorf("failed to create event data: %w", err)
	}
	return nil
}

func (r *EventRepo) EventCreateQuestionBank(ctx context.Context, eventID string, questions []adkutils.QuestionBankQuestion) error {
	err := r.db.SaveQuestionBank(ctx, eventID, questions)
	if err != nil {
		return fmt.Errorf("failed to create question bank: %w", err)
	}
	return nil
}

func (r *EventRepo) EventFindQuestionBank(ctx context.Context, eventID string) (map[string]adkutils.QuestionBankQuestion, error) {
	questions, err := r.db.FindQuestionBank(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to find question bank: %w", err)
	}
	return questions, nil
}
