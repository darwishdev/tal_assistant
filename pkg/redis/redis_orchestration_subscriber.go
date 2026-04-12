package redis

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"tal_assistant/pkg/adk"
	"tal_assistant/pkg/adk/nextquestionextender"
	"tal_assistant/pkg/adk/nextquestionindicator"
	"tal_assistant/pkg/adkutils"

	"github.com/redis/go-redis/v9"
)

type OrchestrationSubscriber struct {
	client    *redis.Client
	emitToUi  func(event string, data interface{})
	adkSvc    adk.ADKServiceInterface
	publisher PublisherInterface
	cache     RedisCacheInterface
}

func NewOrchestrationSubscriber(
	adkSvc adk.ADKServiceInterface,
	publisher PublisherInterface,
	cache RedisCacheInterface,
	emitToUi func(event string, data interface{}),
) *OrchestrationSubscriber {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	return &OrchestrationSubscriber{
		client:    redis.NewClient(&redis.Options{Addr: addr}),
		adkSvc:    adkSvc,
		publisher: publisher,
		emitToUi:  emitToUi,
		cache:     cache,
	}
}

// Run blocks until ctx is cancelled. Call it in a goroutine.
func (s *OrchestrationSubscriber) Run(ctx context.Context) {
	if err := s.client.Ping(ctx).Err(); err != nil {
		log.Printf("[orchestrator] ping failed: %v", err)
		return
	}

	pubsub := s.client.Subscribe(ctx,
		channelSignalDetected,
		channelSignalMapped,
		channelNextQuestionIndicated,
		channelNextQuestionExtended,
	)
	defer pubsub.Close()

	log.Printf("[orchestrator] subscribed to pipeline channels")

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.dispatch(ctx, msg)
		}
	}
}

func (s *OrchestrationSubscriber) dispatch(ctx context.Context, msg *redis.Message) {
	switch msg.Channel {
	case channelSignalDetected:
		var event SignalDetectedEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("[orchestrator] bad signal_detected payload: %v", err)
			return
		}
		s.handleSignalDetected(ctx, event)

	case channelSignalMapped:
		var event SignalMappedEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("[orchestrator] bad signal_mapped payload: %v", err)
			return
		}
		s.handleSignalMapped(ctx, event)

	case channelNextQuestionIndicated:
		var event NextQuestionIndicatedEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("[orchestrator] bad next_question_indicated payload: %v", err)
			return
		}
		s.handleNextQuestionIndicated(ctx, event)

	case channelNextQuestionExtended:
		var event NextQuestionExtendedEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("[orchestrator] bad next_question_extended payload: %v", err)
			return
		}
		s.handleNextQuestionExtended(ctx, event)
	}
}

// handleSignalDetected saves the raw signal, calls the mapper, and publishes signal_mapped.
func (s *OrchestrationSubscriber) handleSignalDetected(ctx context.Context, event SignalDetectedEvent) {
	if event.Signal == "UNCLEAR" {
		log.Printf("[orchestrator] [%s] signal UNCLEAR — skipping", event.InterviewID)
		return
	}

	if err := s.cache.SaveAgentResponse(ctx, event.InterviewID, AgentResponse{
		Agent:  "signaling_agent",
		Input:  event.TranscriptLine,
		Output: event.Signal,
	}); err != nil {
		log.Printf("[orchestrator] [%s] save signaling agent response failed: %v", event.InterviewID, err)
	}

	questionID, err := s.adkSvc.SignalingAgentMapperRun(adkutils.AgentRunRequest{
		Ctx:       ctx,
		SessionID: event.MapperSessionID,
		UserID:    event.UserID,
		Prompt:    event.Signal,
	})
	if err != nil {
		log.Printf("[orchestrator] [%s] mapper run failed: %v", event.InterviewID, err)
		return
	}
	if err := s.cache.SaveAgentResponse(ctx, event.InterviewID, AgentResponse{
		Agent:  "signaling_agent_mapper",
		Input:  event.Signal,
		Output: questionID,
	}); err != nil {
		log.Printf("[orchestrator] [%s] save mapper response failed: %v", event.InterviewID, err)
	}
	if questionID == "UNKNOWN" {
		log.Printf("[orchestrator] [%s] mapper returned UNKNOWN — skipping", event.InterviewID)
		return
	}

	if err := s.publisher.PublishSignalMapped(ctx, SignalMappedEvent{
		InterviewID:        event.InterviewID,
		UserID:             event.UserID,
		Signal:             event.Signal,
		QuestionID:         questionID,
		QAndA:              event.QAndA,
		SignalingSessionID: event.SignalingSessionID,
		MapperSessionID:    event.MapperSessionID,
		IndicatorSessionID: event.IndicatorSessionID,
		ExtenderSessionID:  event.ExtenderSessionID,
	}); err != nil {
		log.Printf("[orchestrator] [%s] publish signal_mapped failed: %v", event.InterviewID, err)
	}
}

// handleSignalMapped persists the transcribed question or answer, updates the current question
// pointer, then calls NQI and publishes next_question_indicated.
func (s *OrchestrationSubscriber) handleSignalMapped(ctx context.Context, event SignalMappedEvent) {
	// persist transcribed question or answer based on signal type
	switch {
	case strings.HasPrefix(event.Signal, "Q:"):
		text := strings.TrimSpace(strings.TrimPrefix(event.Signal, "Q:"))
		if err := s.cache.SaveTranscribedQuestion(ctx, event.InterviewID, event.QuestionID, text); err != nil {
			log.Printf("[orchestrator] [%s] save transcribed question failed: %v", event.InterviewID, err)
		}
	case strings.HasPrefix(event.Signal, "A:"):
		text := strings.TrimSpace(strings.TrimPrefix(event.Signal, "A:"))
		if err := s.cache.SaveAnswer(ctx, event.InterviewID, event.QuestionID, text); err != nil {
			log.Printf("[orchestrator] [%s] save answer failed: %v", event.InterviewID, err)
		}
	}

	if err := s.cache.UpsertCurrentQuestionPointer(ctx, event.InterviewID, event.QuestionID); err != nil {
		log.Printf("[orchestrator] [%s] upsert current question pointer failed: %v", event.InterviewID, err)
	}

	currentQuestion, err := s.cache.FindQuestionByID(ctx, event.InterviewID, event.QuestionID)
	if err != nil {
		log.Printf("[orchestrator] [%s] question %s not found in bank: %v", event.InterviewID, event.QuestionID, err)
		return
	}

	var sb strings.Builder
	for chunk, err := range s.adkSvc.NextQuestionIndicatorRun(adkutils.AgentRunRequest{
		Ctx:       ctx,
		SessionID: event.IndicatorSessionID,
		UserID:    event.UserID,
		Prompt: nextquestionindicator.NextQuestionIndicatorInput{
			CurrentQuestion: *currentQuestion,
			QAndA:           event.QAndA,
		},
	}) {
		if err != nil {
			log.Printf("[orchestrator] [%s] NQI run failed: %v", event.InterviewID, err)
			return
		}
		if chunk == "None" {
			log.Printf("[orchestrator] [%s] NQI run returned null", event.InterviewID)
			return
		}

		s.emitToUi("nqi_chunk_recieved", chunk)
		sb.WriteString(chunk)
	}
	indication := strings.TrimSpace(sb.String())

	if err := s.cache.SaveAgentResponse(ctx, event.InterviewID, AgentResponse{
		Agent:  "next_question_indicator_agent",
		Input:  event.QAndA,
		Output: indication,
	}); err != nil {
		log.Printf("[orchestrator] [%s] save NQI response failed: %v", event.InterviewID, err)
	}

	if err := s.publisher.PublishNextQuestionIndicated(ctx, NextQuestionIndicatedEvent{
		InterviewID:        event.InterviewID,
		UserID:             event.UserID,
		Indication:         indication,
		CurrentQuestionID:  event.QuestionID,
		SignalingSessionID: event.SignalingSessionID,
		MapperSessionID:    event.MapperSessionID,
		IndicatorSessionID: event.IndicatorSessionID,
		ExtenderSessionID:  event.ExtenderSessionID,
	}); err != nil {
		log.Printf("[orchestrator] [%s] publish next_question_indicated failed: %v", event.InterviewID, err)
	}
}

// handleNextQuestionIndicated calls NQE for F:/C: indications and publishes next_question_extended.
func (s *OrchestrationSubscriber) handleNextQuestionIndicated(ctx context.Context, event NextQuestionIndicatedEvent) {
	var questionText string
	var parentQuestionID string
	var questionType string

	switch {
	case strings.HasPrefix(event.Indication, "F:"):
		questionText = strings.TrimSpace(strings.TrimPrefix(event.Indication, "F:"))
		parentQuestionID = event.CurrentQuestionID
		questionType = "followup"
	case strings.HasPrefix(event.Indication, "C:"):
		questionText = strings.TrimSpace(strings.TrimPrefix(event.Indication, "C:"))
		questionType = "change"
	default:
		// None — nothing to do
		return
	}

	question, err := s.adkSvc.NextQuestionExtenderRun(adkutils.AgentRunRequest{
		Ctx:       ctx,
		SessionID: event.ExtenderSessionID,
		UserID:    event.UserID,
		Prompt: nextquestionextender.NextQuestionExtenderInput{
			QuestionText:     questionText,
			ParentQuestionID: parentQuestionID,
		},
	})
	if err != nil {
		log.Printf("[orchestrator] [%s] NQE run failed: %v", event.InterviewID, err)
		return
	}

	nqeOutput, _ := json.Marshal(question)
	if err := s.cache.SaveAgentResponse(ctx, event.InterviewID, AgentResponse{
		Agent:  "next_question_extender",
		Input:  questionText,
		Output: string(nqeOutput),
	}); err != nil {
		log.Printf("[orchestrator] [%s] save NQE response failed: %v", event.InterviewID, err)
	}

	if err := s.publisher.PublishNextQuestionExtended(ctx, NextQuestionExtendedEvent{
		InterviewID:        event.InterviewID,
		UserID:             event.UserID,
		Question:           question,
		Type:               questionType,
		ParentQuestionID:   parentQuestionID,
		SignalingSessionID: event.SignalingSessionID,
		MapperSessionID:    event.MapperSessionID,
		IndicatorSessionID: event.IndicatorSessionID,
	}); err != nil {
		log.Printf("[orchestrator] [%s] publish next_question_extended failed: %v", event.InterviewID, err)
	}
}

// handleNextQuestionExtended persists the new question into the summary and updates the current
// question pointer.
func (s *OrchestrationSubscriber) handleNextQuestionExtended(ctx context.Context, event NextQuestionExtendedEvent) {
	var cacheErr error
	switch event.Type {
	case "followup":
		cacheErr = s.cache.InsertFollowUpQuestion(ctx, event.InterviewID, event.ParentQuestionID, event.Question)
	case "change":
		cacheErr = s.cache.SaveChangeQuestion(ctx, event.InterviewID, event.Question)
	}
	if cacheErr != nil {
		log.Printf("[orchestrator] [%s] persist question %s (%s) failed: %v",
			event.InterviewID, event.Question.ID, event.Type, cacheErr)
		return
	}

	if err := s.cache.UpsertCurrentQuestionPointer(ctx, event.InterviewID, event.Question.ID); err != nil {
		log.Printf("[orchestrator] [%s] upsert current question pointer failed: %v", event.InterviewID, err)
	}

	if err := s.adkSvc.AppendQuestionToSessions(
		ctx,
		event.UserID,
		event.SignalingSessionID,
		event.MapperSessionID,
		event.IndicatorSessionID,
		event.Question,
	); err != nil {
		log.Printf("[orchestrator] [%s] append question to sessions failed: %v", event.InterviewID, err)
	}

	log.Printf("[orchestrator] [%s] %s question %s stored, pointer updated, sessions refreshed",
		event.InterviewID, event.Type, event.Question.ID)
}

func (s *OrchestrationSubscriber) Close() error {
	return s.client.Close()
}
