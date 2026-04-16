package questionbankgenerator

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

// QuestionBankGeneratorInput is passed as Prompt in AgentRunRequest.
type QuestionBankGeneratorInput struct {
	JobTitle          string
	JobDescription    string
	JobRequirements   string
	CandidateName     string
	CandidateSummary  string
	CandidateExperience string
	CandidateEducation  string
	CandidateSkills     string
	UserPrompt          string // optional extra focus from the recruiter
}

// QuestionBankGeneratorState holds the session state variables referenced in the
// instruction template by {job_title}, {job_description}, etc.
type QuestionBankGeneratorState struct {
	JobTitle            string
	JobDescription      string
	JobRequirements     string
	CandidateName       string
	CandidateSummary    string
	CandidateExperience string
	CandidateEducation  string
	CandidateSkills     string
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
		"job_title":            state.JobTitle,
		"job_description":      state.JobDescription,
		"job_requirements":     state.JobRequirements,
		"candidate_name":       state.CandidateName,
		"candidate_summary":    state.CandidateSummary,
		"candidate_experience": state.CandidateExperience,
		"candidate_education":  state.CandidateEducation,
		"candidate_skills":     state.CandidateSkills,
	}
}

func (a *QuestionBankGenerator) Run(
	r *runner.Runner,
	req adkutils.AgentRunRequest,
) ([]adkutils.QuestionBankQuestion, error) {
	input, ok := req.Prompt.(QuestionBankGeneratorInput)
	if !ok {
		return nil, fmt.Errorf("invalid prompt type: expected QuestionBankGeneratorInput, got %T", req.Prompt)
	}

	promptText := "Generate the interview question bank."
	if input.UserPrompt != "" {
		promptText = input.UserPrompt
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
			return nil, err
		}
		if text := adkutils.SessionEventToString(event); text != "" {
			sb.WriteString(text)
		}
	}

	var response struct {
		Questions []adkutils.QuestionBankQuestion `json:"questions"`
	}
	if err := json.Unmarshal([]byte(sb.String()), &response); err != nil {
		return nil, fmt.Errorf("unmarshal question bank: %w", err)
	}
	return response.Questions, nil
}
