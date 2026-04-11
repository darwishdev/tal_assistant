package nextquestionindicator

import (
	"encoding/json"
	"fmt"
	"iter"
	"tal_assistant/pkg/adkutils"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

type NextQuestionIndicatorState struct {
	QuestionBank []adkutils.QuestionBankQuestion
}

// NextQuestionIndicatorInput is passed as Prompt in AgentRunRequest.
type NextQuestionIndicatorInput struct {
	CurrentQuestion adkutils.QuestionBankQuestion
	QAndA           string
}

type NextQuestionIndicator struct {
	agentName string
	llm       *model.LLM
}

func NewNextQuestionIndicator(llm *model.LLM) *NextQuestionIndicator {
	return &NextQuestionIndicator{
		agentName: agentName,
		llm:       llm,
	}
}

func (a *NextQuestionIndicator) NewAgentConfig(m model.LLM) *llmagent.Config {
	return &llmagent.Config{
		Model:       m,
		Name:        agentName,
		Description: agentDescription,
		Instruction: agentInstructions,
	}
}

func (a *NextQuestionIndicator) NewAgentState(state NextQuestionIndicatorState) map[string]any {
	return map[string]any{
		"question_bank": state.QuestionBank,
	}
}

func (a *NextQuestionIndicator) Run(
	r *runner.Runner,
	req adkutils.AgentRunRequest,
) iter.Seq2[string, error] {
	input, ok := req.Prompt.(NextQuestionIndicatorInput)
	if !ok {
		return func(yield func(string, error) bool) {
			yield("", fmt.Errorf("invalid prompt type: expected NextQuestionIndicatorInput, got %T", req.Prompt))
		}
	}

	questionJSON, err := json.MarshalIndent(input.CurrentQuestion, "", "  ")
	if err != nil {
		return func(yield func(string, error) bool) {
			yield("", fmt.Errorf("marshal current question: %w", err))
		}
	}

	text := fmt.Sprintf("Current Question Entity:\n%s\n\nQ&A Exchange:\n%s",
		string(questionJSON), input.QAndA)

	return func(yield func(string, error) bool) {
		events := r.Run(
			req.Ctx,
			req.UserID,
			req.SessionID,
			&genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: text}},
			},
			agent.RunConfig{
				StreamingMode: agent.StreamingModeSSE,
			},
		)

		for event, err := range events {
			if err != nil {
				yield("", err)
				return
			}
			chunk := adkutils.SessionEventToString(event)
			if chunk == "" {
				continue
			}
			if !yield(chunk, nil) {
				return
			}
		}
	}
}
