package nextquestionextender

import (
	"encoding/json"
	"fmt"
	"strings"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

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

type NextQuestionExtenderState struct {
	QuestionBank []adkutils.QuestionBankQuestion
}

// NextQuestionExtenderInput is passed as Prompt in AgentRunRequest.
type NextQuestionExtenderInput struct {
	// QuestionText is the new question text (from F: or C: output of next_question_indicator).
	QuestionText string
	// ParentQuestionID is set only for follow-up questions (F:); empty for question changes (C:).
	ParentQuestionID string
}

type NextQuestionExtender struct {
	agentName string
	llm       *model.LLM
}

func NewNextQuestionExtender(llm *model.LLM) *NextQuestionExtender {
	return &NextQuestionExtender{
		agentName: agentName,
		llm:       llm,
	}
}

func (a *NextQuestionExtender) NewAgentConfig(m model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:        m,
		Name:         agentName,
		Description:  agentDescription,
		Instruction:  agentInstructions,
		OutputSchema: questionBankQuestionSchema,
	}
}

func (a *NextQuestionExtender) NewAgentState(state NextQuestionExtenderState) map[string]any {
	return map[string]any{
		"question_bank": state.QuestionBank,
	}
}

func (a *NextQuestionExtender) Run(
	r *runner.Runner,
	req adkutils.AgentRunRequest,
) (adkutils.QuestionBankQuestion, error) {
	input, ok := req.Prompt.(NextQuestionExtenderInput)
	if !ok {
		return adkutils.QuestionBankQuestion{}, fmt.Errorf("invalid prompt type: expected NextQuestionExtenderInput, got %T", req.Prompt)
	}

	var promptText string
	if input.ParentQuestionID != "" {
		promptText = fmt.Sprintf("Question text: %s\nParent question ID: %s", input.QuestionText, input.ParentQuestionID)
	} else {
		promptText = fmt.Sprintf("Question text: %s", input.QuestionText)
	}

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

	var sb strings.Builder
	for event, err := range events {
		if err != nil {
			return adkutils.QuestionBankQuestion{}, err
		}
		if text := adkutils.SessionEventToString(event); text != "" {
			sb.WriteString(text)
		}
	}

	var question adkutils.QuestionBankQuestion
	if err := json.Unmarshal([]byte(sb.String()), &question); err != nil {
		return adkutils.QuestionBankQuestion{}, fmt.Errorf("unmarshal question: %w", err)
	}
	return question, nil
}
