package usecase

import (
	"context"
	"fmt"
	"tal_assistant/app/event/dto"
	"tal_assistant/pkg/adkutils"
	"tal_assistant/pkg/atsclient"
	"tal_assistant/pkg/workableclient"
)

type EventLoginResponse struct {
	ATSLogin *atsclient.LoginResponse `json:"ats_login"`
	Member   *workableclient.Member   `json:"member"`
}

func (u *EventUseCase) EventList(req dto.EventListRequest) (*dto.EventListResponse, error) {
	workableRequest := u.adapter.EventListRequestWorkableFromDTO(req)
	events, err := u.workableClient.ListEvents(workableRequest)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	response := u.adapter.EventListResponseDTOFromWorkable(events)
	return response, nil
}

func (u *EventUseCase) EventFind(ctx context.Context, eventID string) (*dto.EventFindResponse, error) {
	// workableRequest := u.adapter.EventListRequestWorkableFromDTO(req)
	// fetch from repo first
	questionBankQuestions := []adkutils.QuestionBankQuestion{}
	questions, err := u.repo.EventFindQuestionBank(ctx, eventID)
	if err != nil {
		fmt.Printf("failed to find question bank for eventID: %s, error: %v", eventID, err)
	}
	if err == nil && len(questions) > 0 {
		questionBankQuestions = u.adapter.QuestionBankListFromMap(questions)
	}
	cachedEvent, err := u.repo.EventFind(ctx, eventID)
	if err != nil {
		fmt.Printf("failed to fetch event from cache: %v", err)
	}
	// response := &dto.EventFindResponse{
	// 	QuestionBank: questionBankQuestions,
	// }
	if err == nil && cachedEvent != nil {
		fmt.Printf("cache hit for eventID: %s\n", eventID)
		if len(cachedEvent) > 0 {
			response, err := u.adapter.EventFindResponseDTOFromCache(cachedEvent)
			if err == nil {
				return response, nil
			}
			fmt.Printf("failed to parse cached event data: %v", err)
		}
	}
	event, err := u.workableClient.EventFind(eventID)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	response := u.adapter.EventFindResponseDTOFromWorkable(event)
	response.QuestionBank = questionBankQuestions
	// cache the response for future use
	eventData, err := u.adapter.EventFindCacheDataFromWorkable(event)
	if err != nil {
		fmt.Printf("failed to serialize event data for caching: %v", err)
		return response, nil
	}
	err = u.repo.EventCreate(ctx, eventID, eventData)
	if err != nil {
		fmt.Printf("failed to cache event data: %v", err)
	}

	// find q bank for this event

	return response, nil
}
