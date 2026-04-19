package repo

import (
	"context"
	"tal_assistant/pkg/adkutils"
	redispkg "tal_assistant/pkg/redis"
)

type EventRepoInterface interface {
	EventFind(ctx context.Context, eventID string) ([]byte, error)
	EventCreate(ctx context.Context, eventID string, eventData []byte) error
	EventFindQuestionBank(ctx context.Context, eventID string) (map[string]adkutils.QuestionBankQuestion, error)
}
type EventRepo struct {
	db redispkg.RedisCacheInterface
}

func NewEventRepo(db redispkg.RedisCacheInterface) *EventRepo {
	return &EventRepo{
		db: db,
	}
}
