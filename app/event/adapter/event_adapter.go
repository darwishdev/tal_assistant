package adapter

import (
	"tal_assistant/app/event/dto"
	"tal_assistant/pkg/adkutils"
	"tal_assistant/pkg/workableclient"
)

type EventAdapterInterface interface {
	EventListRequestWorkableFromDTO(req dto.EventListRequest) workableclient.ListEventsOptions
	EventListResponseDTOFromWorkable(events []workableclient.Event) *dto.EventListResponse
	EventFindResponseDTOFromWorkable(event *workableclient.EventFindResult) *dto.EventFindResponse
	EventFindResponseDTOFromCache(cachedData []byte) (*dto.EventFindResponse, error)
	EventFindCacheDataFromWorkable(event *workableclient.EventFindResult) ([]byte, error)
	QuestionBankListFromMap(questions map[string]adkutils.QuestionBankQuestion) []adkutils.QuestionBankQuestion
}

type EventAdapter struct {
}

func NewEventAdapter() *EventAdapter {
	return &EventAdapter{}
}
