package adk

import (
	"iter"
	"tal_assistant/pkg/adk/nextquestionindicator"
	"tal_assistant/pkg/adkutils"
)

func (s *ADKService) NextQuestionIndicatorRun(req adkutils.AgentRunRequest) iter.Seq2[string, error] {
	return s.nextQuestionIndicator.Run(s.nextQuestionIndicatorRunner, req)
}

func (s *ADKService) NewNextQuestionIndicatorState(req nextquestionindicator.NextQuestionIndicatorState) map[string]any {
	return s.nextQuestionIndicator.NewAgentState(req)
}
