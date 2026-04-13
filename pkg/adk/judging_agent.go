package adk

import (
	"iter"
	"tal_assistant/pkg/adk/judgingagent"
	"tal_assistant/pkg/adkutils"
)

func (s *ADKService) JudgingAgentRun(req adkutils.AgentRunRequest) iter.Seq2[string, error] {
	return s.judgingAgent.Run(s.judgingAgentRunner, req)
}

func (s *ADKService) NewJudgingAgentState(req judgingagent.JudgingAgentState) map[string]any {
	return s.judgingAgent.NewAgentState(req)
}
