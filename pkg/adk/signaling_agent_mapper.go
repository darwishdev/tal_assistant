package adk

import (
	"tal_assistant/pkg/adk/signalingagentmapper"
	"tal_assistant/pkg/adkutils"
)

func (s *ADKService) SignalingAgentMapperRun(req adkutils.AgentRunRequest) (string, error) {
	return s.signalingAgentMapper.Run(s.signalingAgentMapperRunner, req)
}

func (s *ADKService) NewSignalingAgentMapperState(req signalingagentmapper.SignalingAgentMapperState) map[string]any {
	return s.signalingAgentMapper.NewAgentState(req)
}
