package nextquestionindicator

import (
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
	// Prompt is now a plain string (either current question JSON or candidate's answer)
	text, ok := req.Prompt.(string)
	if !ok {
		return func(yield func(string, error) bool) {
			yield("", fmt.Errorf("invalid prompt type: expected string, got %T", req.Prompt))
		}
	}

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
