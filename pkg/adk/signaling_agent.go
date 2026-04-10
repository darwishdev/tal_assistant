package adk

import (
	"iter"
	"tal_assistant/pkg/adk/signalingagent"
	"tal_assistant/pkg/adkutils"
)

func (s *ADKService) SignalingAgentRun(req adkutils.AgentRunRequest) iter.Seq2[string, error] {
	return s.singalinAgent.Run(s.signalingAgentRunner, req)
}

func (s *ADKService) NewSignalingAgentState(req signalingagent.SignalingAgentState) map[string]any {
	return s.singalinAgent.NewAgentState(req)
}
