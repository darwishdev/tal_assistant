package adk

import (
	"tal_assistant/pkg/adk/nextquestionextender"
	"tal_assistant/pkg/adkutils"
)

func (s *ADKService) NextQuestionExtenderRun(req adkutils.AgentRunRequest) (adkutils.QuestionBankQuestion, error) {
	return s.nextQuestionExtender.Run(s.nextQuestionExtenderRunner, req)
}

func (s *ADKService) NewNextQuestionExtenderState(req nextquestionextender.NextQuestionExtenderState) map[string]any {
	return s.nextQuestionExtender.NewAgentState(req)
}
