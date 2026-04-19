package adapter

import (
	"encoding/json"
	"fmt"
	"tal_assistant/app/event/dto"
	"tal_assistant/pkg/adkutils"
	"tal_assistant/pkg/workableclient"
)

func (a *EventAdapter) EventListRequestWorkableFromDTO(req dto.EventListRequest) workableclient.ListEventsOptions {
	eventType := req.EventType
	if eventType == "" {
		eventType = "interview"
	}
	limit := req.Limit
	if limit == 0 {
		limit = 10
	}
	return workableclient.ListEventsOptions{
		MemberID:  req.MemberID,
		EventType: eventType,
		Limit:     limit,
	}
}

func (a *EventAdapter) EventListResponseDTOFromWorkable(events []workableclient.Event) *dto.EventListResponse {
	if len(events) == 0 {
		return &dto.EventListResponse{
			Events: []dto.EventListRow{},
		}
	}
	eventDTOs := make([]dto.EventListRow, len(events))
	for i, event := range events {
		eventDTOs[i] = dto.EventListRow{
			ID:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartsAt:    event.StartsAt,
			EndsAt:      event.EndsAt,
			Cancelled:   event.Cancelled,
		}
	}
	return &dto.EventListResponse{
		Events: eventDTOs,
	}
}

func (a *EventAdapter) EventFindResponseDTOFromWorkable(result *workableclient.EventFindResult) *dto.EventFindResponse {
	if result == nil {
		return nil
	}
	return &dto.EventFindResponse{
		Event:     a.eventFromWorkable(result.Event),
		Candidate: a.candidateFromWorkable(result.Candidate),
		Job:       a.jobFromWorkable(result.Job),
	}
}
func (a *EventAdapter) EventFindResponseDTOFromCache(cachedData []byte) (*dto.EventFindResponse, error) {
	if len(cachedData) == 0 {
		return nil, nil
	}
	var cachedEvent dto.EventFindResponse
	err := json.Unmarshal(cachedData, &cachedEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached event data: %w", err)
	}
	return &cachedEvent, nil
}
func (a *EventAdapter) EventFindCacheDataFromWorkable(event *workableclient.EventFindResult) ([]byte, error) {
	if event == nil {
		return nil, nil
	}
	dtoEvent := a.EventFindResponseDTOFromWorkable(event)
	data, err := json.Marshal(dtoEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data for caching: %w", err)
	}
	return data, nil
}

func (a *EventAdapter) QuestionBankListFromMap(questions map[string]adkutils.QuestionBankQuestion) []adkutils.QuestionBankQuestion {
	if len(questions) == 0 {
		return []adkutils.QuestionBankQuestion{}
	}
	questionList := make([]adkutils.QuestionBankQuestion, 0, len(questions))
	for _, question := range questions {
		questionList = append(questionList, question)
	}
	return questionList
}
