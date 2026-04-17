package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"tal_assistant/pkg/adk"
	"tal_assistant/pkg/adk/nextquestionextender"
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
	redisUrl string,
	adkSvc adk.ADKServiceInterface,
	publisher PublisherInterface,
	cache RedisCacheInterface,
	emitToUi func(event string, data interface{}),
) *OrchestrationSubscriber {
	if redisUrl == "" {
		addr := os.Getenv("REDIS_URL")
		if addr == "" {
			addr = "localhost:6379"
		}
		redisUrl = addr
	}
	return &OrchestrationSubscriber{
		client:    redis.NewClient(&redis.Options{Addr: redisUrl}),
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

	// Before calling the mapper agent, check if we can match the signal question
	// directly against cached questions to avoid unnecessary AI calls
	var questionID string
	if strings.HasPrefix(event.Signal, "Q:") {
		signalQuestionText := strings.TrimSpace(strings.TrimPrefix(event.Signal, "Q:"))

		// First check if it matches the current question
		if currentPointer, err := s.cache.FindCurrentQuestionPointer(ctx, event.InterviewID); err == nil && currentPointer != "" {
			if currentQ, err := s.cache.FindQuestionByID(ctx, event.InterviewID, currentPointer); err == nil {
				if strings.EqualFold(strings.TrimSpace(currentQ.Question), signalQuestionText) {
					questionID = currentPointer
					log.Printf("[orchestrator] [%s] signal matched current question → %s (skipped mapper)", event.InterviewID, questionID)
				}
			}
		}

		// If no match with current question, check all questions in the bank
		if questionID == "" {
			if summary, err := s.cache.FindInterviewSummary(ctx, event.InterviewID); err == nil {
				for _, qa := range summary.Questions {
					if strings.EqualFold(strings.TrimSpace(qa.Question.Question), signalQuestionText) {
						questionID = qa.Question.ID
						log.Printf("[orchestrator] [%s] signal matched question bank → %s (skipped mapper)", event.InterviewID, questionID)
						break
					}
				}
			}
		}
	}

	// If no direct match found, call the mapper agent
	if questionID == "" {
		var err error
		questionID, err = s.adkSvc.SignalingAgentMapperRun(adkutils.AgentRunRequest{
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
	}

	if questionID == "UNKNOWN" {
		log.Printf("[orchestrator] [%s] mapper returned UNKNOWN — skipping", event.InterviewID)
		return
	}

	// Update the active question pointer immediately — before NQI runs,
	// so the UI and any concurrent readers always know the current question.
	if err := s.cache.UpsertCurrentQuestionPointer(ctx, event.InterviewID, questionID); err != nil {
		log.Printf("[orchestrator] [%s] upsert current question pointer failed: %v", event.InterviewID, err)
	}

	// Emit the current question text to the recruiter UI.
	if q, err := s.cache.FindQuestionByID(ctx, event.InterviewID, questionID); err == nil {
		s.emitToUi("current_question", q.Question)
		log.Printf("[orchestrator] [%s] active question → %s: %q", event.InterviewID, questionID, q.Question)
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
		JudgingSessionID:   event.JudgingSessionID,
	}); err != nil {
		log.Printf("[orchestrator] [%s] publish signal_mapped failed: %v", event.InterviewID, err)
	}
}

// handleSignalMapped persists the transcribed question or answer, updates the current question
// pointer, then calls NQI and publishes next_question_indicated.
func (s *OrchestrationSubscriber) handleSignalMapped(ctx context.Context, event SignalMappedEvent) {
	currentQuestion, err := s.cache.FindQuestionByID(ctx, event.InterviewID, event.QuestionID)
	if err != nil {
		log.Printf("[orchestrator] [%s] question %s not found in bank: %v", event.InterviewID, event.QuestionID, err)
		return
	}

	// Handle signal based on type
	switch {
	case strings.HasPrefix(event.Signal, "Q:"):
		// Question signal: persist transcribed question and send it to NQI (agent should not respond)
		text := strings.TrimSpace(strings.TrimPrefix(event.Signal, "Q:"))
		if err := s.cache.SaveTranscribedQuestion(ctx, event.InterviewID, event.QuestionID, text); err != nil {
			log.Printf("[orchestrator] [%s] save transcribed question failed: %v", event.InterviewID, err)
		}

		// Send current question to NQI (no response expected)
		questionJSON, _ := json.MarshalIndent(currentQuestion, "", "  ")
		prompt := fmt.Sprintf("Current Question Entity:\n%s", string(questionJSON))

		for _, err := range s.adkSvc.NextQuestionIndicatorRun(adkutils.AgentRunRequest{
			Ctx:       ctx,
			SessionID: event.IndicatorSessionID,
			UserID:    event.UserID,
			Prompt:    prompt,
		}) {
			if err != nil {
				log.Printf("[orchestrator] [%s] NQI question send failed: %v", event.InterviewID, err)
				return
			}
			// Consume and discard any response (agent should not respond to questions)
		}
		log.Printf("[orchestrator] [%s] current question sent to NQI session", event.InterviewID)

		// Send question to Judging Agent (no response expected)
		judgingPrompt := fmt.Sprintf("Q: %s", text)
		for _, err := range s.adkSvc.JudgingAgentRun(adkutils.AgentRunRequest{
			Ctx:       ctx,
			SessionID: event.JudgingSessionID,
			UserID:    event.UserID,
			Prompt:    judgingPrompt,
		}) {
			if err != nil {
				log.Printf("[orchestrator] [%s] judging agent question send failed: %v", event.InterviewID, err)
				return
			}
			// Consume and discard any response (agent should not respond to questions)
		}
		log.Printf("[orchestrator] [%s] question sent to judging agent session", event.InterviewID)
		return

	case strings.HasPrefix(event.Signal, "A:"):
		// Answer signal: check if it's an answer-end signal (ends with semicolon)
		text := strings.TrimSpace(strings.TrimPrefix(event.Signal, "A:"))
		isAnswerEnd := strings.HasSuffix(text, ";")

		// Remove the semicolon from the answer text if present
		if isAnswerEnd {
			text = strings.TrimSuffix(text, ";")
			text = strings.TrimSpace(text)
		}

		// Always save the answer (whether partial or complete)
		if err := s.cache.SaveAnswer(ctx, event.InterviewID, event.QuestionID, text); err != nil {
			log.Printf("[orchestrator] [%s] save answer failed: %v", event.InterviewID, err)
		}

		// If not an answer-end signal, skip the judging and NQI flow
		if !isAnswerEnd {
			log.Printf("[orchestrator] [%s] answer in progress (no semicolon) — skipping judging/NQI", event.InterviewID)
			return
		}

		log.Printf("[orchestrator] [%s] answer complete (semicolon detected) — proceeding with judging/NQI", event.InterviewID)
	}

	// Only process NQI decision when we have a complete answer signal (with semicolon)
	if !strings.HasPrefix(event.Signal, "A:") {
		return
	}

	// Extract answer text (semicolon already removed above)
	answerText := strings.TrimSpace(strings.TrimPrefix(event.Signal, "A:"))
	if strings.HasSuffix(answerText, ";") {
		answerText = strings.TrimSuffix(answerText, ";")
		answerText = strings.TrimSpace(answerText)
	}
	prompt := fmt.Sprintf("Candidate's Answer:\n%s", answerText)

	var sb strings.Builder
	for chunk, err := range s.adkSvc.NextQuestionIndicatorRun(adkutils.AgentRunRequest{
		Ctx:       ctx,
		SessionID: event.IndicatorSessionID,
		UserID:    event.UserID,
		Prompt:    prompt,
	}) {
		if err != nil {
			log.Printf("[orchestrator] [%s] NQI run failed: %v", event.InterviewID, err)
			return
		}
		if chunk == "None" {
			log.Printf("[orchestrator] [%s] NQI returned None — no action needed", event.InterviewID)
			return
		}

		// The ADK SDK streams incremental chunks then fires one final event
		// containing the ENTIRE accumulated text as a single chunk.
		// Detect it: if this chunk equals everything we already accumulated,
		// it is the duplicate final emission — drop it and stop.
		if sb.Len() > 0 && chunk == sb.String() {
			log.Printf("[orchestrator] [%s] NQI dedup: suppressed final re-emission (len=%d)", event.InterviewID, len(chunk))
			break
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

	// ── Judging Agent: Evaluate the Q&A pair ──────────────────────────────
	// Send answer to Judging Agent and collect JSON judgment
	judgingPrompt := fmt.Sprintf("A: %s", answerText)
	var judgingSb strings.Builder
	for chunk, err := range s.adkSvc.JudgingAgentRun(adkutils.AgentRunRequest{
		Ctx:       ctx,
		SessionID: event.JudgingSessionID,
		UserID:    event.UserID,
		Prompt:    judgingPrompt,
	}) {
		if err != nil {
			log.Printf("[orchestrator] [%s] judging agent run failed: %v", event.InterviewID, err)
			break
		}
		// Handle deduplication: ADK SDK sends incremental chunks then a final full chunk
		if judgingSb.Len() > 0 && chunk == judgingSb.String() {
			log.Printf("[orchestrator] [%s] judging agent dedup: suppressed final re-emission (len=%d)", event.InterviewID, len(chunk))
			break
		}
		judgingSb.WriteString(chunk)
	}
	judgmentJSON := strings.TrimSpace(judgingSb.String())

	// Parse and save the judgment
	if judgmentJSON != "" {
		var judgment Judgment
		if err := json.Unmarshal([]byte(judgmentJSON), &judgment); err != nil {
			log.Printf("[orchestrator] [%s] parse judging agent JSON failed: %v\nJSON: %s", event.InterviewID, err, judgmentJSON)
		} else {
			if err := s.cache.SaveJudgment(ctx, event.InterviewID, event.QuestionID, &judgment); err != nil {
				log.Printf("[orchestrator] [%s] save judgment failed: %v", event.InterviewID, err)
			} else {
				passStatus := "FAIL"
				if judgment.Pass {
					passStatus = "PASS"
				}
				log.Printf("[orchestrator] [%s] judgment saved — question=%s score=%d/%d %s verdict=%q",
					event.InterviewID, event.QuestionID, judgment.Score, 100, passStatus, judgment.Verdict)

				// Emit judgment to UI for real-time feedback
				s.emitToUi("judgment_received", map[string]interface{}{
					"question_id": event.QuestionID,
					"judgment":    judgment,
				})

				// Fetch and emit updated interview summary to UI
				if summary, err := s.cache.FindInterviewSummary(ctx, event.InterviewID); err != nil {
					log.Printf("[orchestrator] [%s] fetch interview summary for UI failed: %v", event.InterviewID, err)
				} else {
					s.emitToUi("interview_summary_updated", summary)
					log.Printf("[orchestrator] [%s] interview summary emitted to UI — questions=%d", event.InterviewID, len(summary.Questions))
				}
			}

			// Save the judgment as an agent response for audit trail
			if err := s.cache.SaveAgentResponse(ctx, event.InterviewID, AgentResponse{
				Agent:  "judging_agent",
				Input:  event.QAndA,
				Output: judgmentJSON,
			}); err != nil {
				log.Printf("[orchestrator] [%s] save judging agent response failed: %v", event.InterviewID, err)
			}
		}
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
		JudgingSessionID:   event.JudgingSessionID,
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
		JudgingSessionID:   event.JudgingSessionID,
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

	// Surface the new question text to the recruiter UI immediately.
	s.emitToUi("current_question", event.Question.Question)
	log.Printf("[orchestrator] [%s] new %s question surfaced → %s", event.InterviewID, event.Type, event.Question.ID)

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
