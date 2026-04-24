package adk

import (
	"tal_assistant/pkg/adk/summarizeragent"
	"tal_assistant/pkg/adkutils"
)

func (s *ADKService) SummarizerAgentRun(req adkutils.AgentRunRequest) (*summarizeragent.InterviewSummaryReport, error) {
	return s.summarizerAgent.Run(s.summarizerAgentRunner, req)
}

func (s *ADKService) NewSummarizerAgentState(req summarizeragent.SummarizerAgentState) map[string]any {
	return s.summarizerAgent.NewAgentState(req)
}
