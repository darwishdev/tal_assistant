package adk

import (
	"tal_assistant/pkg/adk/questionbankgenerator"
	"tal_assistant/pkg/adkutils"
)

func (s *ADKService) QuestionBankGeneratorRun(req adkutils.AgentRunRequest) ([]adkutils.QuestionBankQuestion, error) {
	return s.questionBankGenerator.Run(s.questionBankGeneratorRunner, req)
}

func (s *ADKService) NewQuestionBankGeneratorState(req questionbankgenerator.QuestionBankGeneratorState) map[string]any {
	return s.questionBankGenerator.NewAgentState(req)
}
