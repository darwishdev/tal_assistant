package summarizeragent

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

// SummarizerAgentState holds session-level state injected into the instruction template.
type SummarizerAgentState struct {
	InterviewContext string
}

// SummarizerInput is passed as Prompt in AgentRunRequest.
type SummarizerInput struct {
	// Full Q&A transcript as a human-readable formatted string.
	Transcript string
}

// AspectScores holds per-dimension evaluation scores.
type AspectScores struct {
	TechnicalKnowledge  int `json:"technical_knowledge"`
	ProblemSolving      int `json:"problem_solving"`
	Communication       int `json:"communication"`
	CultureFit          int `json:"culture_fit"`
	ExperienceRelevance int `json:"experience_relevance"`
}

// InterviewSummaryReport is the structured output of the summarizer agent.
type InterviewSummaryReport struct {
	OverallScore       int          `json:"overall_score"`
	HireRecommendation string       `json:"hire_recommendation"`
	Summary            string       `json:"summary"`
	AspectScores       AspectScores `json:"aspect_scores"`
	Strengths          []string     `json:"strengths"`
	Weaknesses         []string     `json:"weaknesses"`
	RedFlags           []string     `json:"red_flags"`
	StandoutMoments    []string     `json:"standout_moments"`
	DevelopmentAreas   []string     `json:"development_areas"`
	InterviewNotes     string       `json:"interview_notes"`
}

// outputSchema describes the JSON structure the agent must produce.
var outputSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"overall_score":       {Type: genai.TypeInteger},
		"hire_recommendation": {Type: genai.TypeString},
		"summary":             {Type: genai.TypeString},
		"aspect_scores": {
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"technical_knowledge":  {Type: genai.TypeInteger},
				"problem_solving":      {Type: genai.TypeInteger},
				"communication":        {Type: genai.TypeInteger},
				"culture_fit":          {Type: genai.TypeInteger},
				"experience_relevance": {Type: genai.TypeInteger},
			},
			Required: []string{"technical_knowledge", "problem_solving", "communication", "culture_fit", "experience_relevance"},
		},
		"strengths":         {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
		"weaknesses":        {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
		"red_flags":         {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
		"standout_moments":  {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
		"development_areas": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
		"interview_notes":   {Type: genai.TypeString},
	},
	Required: []string{
		"overall_score", "hire_recommendation", "summary",
		"aspect_scores", "strengths", "weaknesses",
		"red_flags", "standout_moments", "development_areas", "interview_notes",
	},
}

// SummarizerAgent wraps the LLM agent and its runner.
type SummarizerAgent struct {
	agentName string
	llm       *model.LLM
}

func NewSummarizerAgent(llm *model.LLM) *SummarizerAgent {
	return &SummarizerAgent{
		agentName: agentName,
		llm:       llm,
	}
}

func (a *SummarizerAgent) NewAgentConfig(m model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:        m,
		Name:         agentName,
		Description:  agentDescription,
		Instruction:  agentInstructions,
		OutputSchema: outputSchema,
	}
}

func (a *SummarizerAgent) NewAgentState(state SummarizerAgentState) map[string]any {
	return map[string]any{
		"interview_context": state.InterviewContext,
	}
}

func (a *SummarizerAgent) Run(
	r *runner.Runner,
	req adkutils.AgentRunRequest,
) (*InterviewSummaryReport, error) {
	log.Printf("[summarizer-agent] Run called — sessionID=%s userID=%s", req.SessionID, req.UserID)

	input, ok := req.Prompt.(SummarizerInput)
	if !ok {
		return nil, fmt.Errorf("invalid prompt type: expected SummarizerInput, got %T", req.Prompt)
	}

	events := r.Run(
		req.Ctx,
		req.UserID,
		req.SessionID,
		&genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: input.Transcript}},
		},
		agent.RunConfig{
			StreamingMode: agent.StreamingModeSSE,
		},
	)

	var sb strings.Builder
	for event, err := range events {
		if err != nil {
			return nil, fmt.Errorf("summarizer agent stream error: %w", err)
		}
		chunk := adkutils.SessionEventToString(event)
		if chunk != "" {
			sb.WriteString(chunk)
		}
	}

	raw := strings.TrimSpace(sb.String())
	log.Printf("[summarizer-agent] raw output len=%d", len(raw))

	// Strip any accidental markdown fences
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var report InterviewSummaryReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, fmt.Errorf("failed to parse summarizer output: %w (raw=%q)", err, raw)
	}

	log.Printf("[summarizer-agent] report parsed — overall_score=%d recommendation=%q",
		report.OverallScore, report.HireRecommendation)

	return &report, nil
}
