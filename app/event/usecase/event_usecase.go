package usecase

import (
	"tal_assistant/app/event/adapter"
	"tal_assistant/app/event/repo"
	"tal_assistant/pkg/adkutils"
	redispkg "tal_assistant/pkg/redis"
	"tal_assistant/pkg/workableclient"
)

type ListSourcesResonse struct {
	AudioDevices []string `json:"audio_devices"`
}
type EventUseCaseInterface interface {
	EventList(memberID string) ([]workableclient.Event, error)
	EventFind(eventID string) (workableclient.Event, error)
	EventQuestionBankPromptSave(eventID string, questions []adkutils.QuestionBankQuestion) error
	EventQuestionBankCreate(eventID string, questions []adkutils.QuestionBankQuestion) error
}
type EventUseCase struct {
	workableClient workableclient.ClientInterface
	adapter        adapter.EventAdapterInterface
	repo           repo.EventRepoInterface
}

func NewEventUseCase(
	workableClient workableclient.ClientInterface,
	redisCache redispkg.RedisCacheInterface,
) *EventUseCase {
	adapter := adapter.NewEventAdapter()
	repo := repo.NewEventRepo(redisCache)
	return &EventUseCase{
		workableClient: workableClient,
		adapter:        adapter,
		repo:           repo,
	}
}
