package questionbankgenerator

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

// QuestionBankGeneratorInput is passed as Prompt in AgentRunRequest.
type QuestionBankGeneratorInput struct {
	EventDataJSON string // Full Workable EventFindResult as JSON string
	UserPrompt    string // optional extra focus from the recruiter
}

// QuestionBankGeneratorState holds the session state variables referenced in the
// instruction template.
type QuestionBankGeneratorState struct {
	EventDataJSON string // Full Workable EventFindResult as JSON string
}

var questionBankQuestionSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"id":                     {Type: genai.TypeString},
		"category":               {Type: genai.TypeString},
		"difficulty":             {Type: genai.TypeString},
		"estimated_time_minutes": {Type: genai.TypeInteger},
		"evaluation_criteria": {
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"bonus_points": {Type: genai.TypeString},
					"must_mention": {Type: genai.TypeString},
				},
				Required: []string{"bonus_points", "must_mention"},
			},
		},
		"followup_triggers": {
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"condition": {Type: genai.TypeString},
					"follow_up": {Type: genai.TypeString},
				},
				Required: []string{"condition", "follow_up"},
			},
		},
		"ideal_answer_keywords": {Type: genai.TypeString},
		"pass_treshold":         {Type: genai.TypeNumber},
		"question":              {Type: genai.TypeString},
	},
	Required: []string{
		"id", "category", "difficulty", "estimated_time_minutes",
		"evaluation_criteria", "followup_triggers",
		"ideal_answer_keywords", "pass_treshold", "question",
	},
}

var outputSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"questions": {
			Type:  genai.TypeArray,
			Items: questionBankQuestionSchema,
		},
	},
	Required: []string{"questions"},
}

type QuestionBankGenerator struct {
	agentName string
	llm       *model.LLM
}

func NewQuestionBankGenerator(llm *model.LLM) *QuestionBankGenerator {
	return &QuestionBankGenerator{
		agentName: agentName,
		llm:       llm,
	}
}

func (a *QuestionBankGenerator) NewAgentConfig(m model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:        m,
		Name:         agentName,
		Description:  agentDescription,
		Instruction:  agentInstructions,
		OutputSchema: outputSchema,
	}
}

func (a *QuestionBankGenerator) NewAgentState(state QuestionBankGeneratorState) map[string]any {
	return map[string]any{
		"event_data_json": state.EventDataJSON,
	}
}

func (a *QuestionBankGenerator) Run(
	r *runner.Runner,
	req adkutils.AgentRunRequest,
) ([]adkutils.QuestionBankQuestion, error) {
	log.Printf("[qbgen-agent] Run called for sessionID=%s userID=%s", req.SessionID, req.UserID)
	
	input, ok := req.Prompt.(QuestionBankGeneratorInput)
	if !ok {
		err := fmt.Errorf("invalid prompt type: expected QuestionBankGeneratorInput, got %T", req.Prompt)
		log.Printf("[qbgen-agent] ERROR: %v", err)
		return nil, err
	}

	promptBuilder := strings.Builder{}
	promptBuilder.WriteString("Generate the interview question bank based on the following data:\n\n")
	if input.EventDataJSON != "" {
		log.Printf("[qbgen-agent] EventDataJSON length: %d bytes", len(input.EventDataJSON))
		promptBuilder.WriteString("Event Data (JSON):\n```json\n" + input.EventDataJSON + "\n```\n\n")
	} else {
		log.Printf("[qbgen-agent] WARNING: EventDataJSON is empty")
	}
	if input.UserPrompt != "" {
		log.Printf("[qbgen-agent] UserPrompt: %s", input.UserPrompt)
		promptBuilder.WriteString("Additional Focus: " + input.UserPrompt + "\n")
	}

	promptText := strings.TrimSpace(promptBuilder.String())
	log.Printf("[qbgen-agent] Built prompt, total length: %d bytes", len(promptText))
	
	// Log first 500 chars of prompt for debugging
	if len(promptText) > 500 {
		log.Printf("[qbgen-agent] Prompt preview (first 500 chars): %s...", promptText[:500])
	} else {
		log.Printf("[qbgen-agent] Full prompt: %s", promptText)
	}

	log.Printf("[qbgen-agent] Calling agent runner...")
	events := r.Run(
		req.Ctx,
		req.UserID,
		req.SessionID,
		&genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: promptText}},
		},
		agent.RunConfig{
			StreamingMode: agent.StreamingModeNone,
		},
	)

	log.Printf("[qbgen-agent] Processing events from runner...")
	var sb strings.Builder
	eventCount := 0
	for event, err := range events {
		if err != nil {
			log.Printf("[qbgen-agent] ERROR in event stream: %v", err)
			return nil, err
		}
		eventCount++
		if text := adkutils.SessionEventToString(event); text != "" {
			sb.WriteString(text)
			log.Printf("[qbgen-agent] Event %d: received %d bytes", eventCount, len(text))
		}
	}
	
	rawResponse := sb.String()
	log.Printf("[qbgen-agent] Received %d events, total response length: %d bytes", eventCount, len(rawResponse))
	
	// Log the raw response
	if len(rawResponse) > 1000 {
		log.Printf("[qbgen-agent] Raw response (first 1000 chars): %s...", rawResponse[:1000])
	} else {
		log.Printf("[qbgen-agent] Raw response: %s", rawResponse)
	}

	log.Printf("[qbgen-agent] Attempting to unmarshal response...")
	var response struct {
		Questions []adkutils.QuestionBankQuestion `json:"questions"`
	}
	if err := json.Unmarshal([]byte(rawResponse), &response); err != nil {
		log.Printf("[qbgen-agent] ERROR: unmarshal failed: %v", err)
		log.Printf("[qbgen-agent] Failed to parse response as JSON. Raw response: %s", rawResponse)
		return nil, fmt.Errorf("unmarshal question bank: %w", err)
	}
	
	log.Printf("[qbgen-agent] Successfully unmarshaled %d questions", len(response.Questions))
	return response.Questions, nil
}
