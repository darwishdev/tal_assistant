package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"tal_assistant/pkg/timeutils"

	pb "github.com/a2aproject/a2a-go/a2apb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SignalResult carries the full detected Q or A, plus an optional NQI
// suggestion that the signal detector may have inferred automatically.
type SignalResult struct {
	Timestamp    string
	Signal       string // "question" or "answer"
	Text         string
	SigLine      string
	NextQuestion string // set when signal detector ran NQI inline
	NQIRationale string
}

// NextQuestionResult carries the suggested follow-up question.
type NextQuestionResult struct {
	NextQuestion string `json:"next_question"`
	Rationale    string `json:"rationale"`
}

type ADKServiceInterface interface {
	ExtractSignalsStream(speaker, transcript string, timestampMs int64) (*SignalResult, error)
	InferNextQuestion(question, answer string) (*NextQuestionResult, error)
	InferNextQuestionManual(prompt, transcriptSnippet string) (*NextQuestionResult, error)
	Reset()
	Close() error
}

type ADKService struct {
	sigConn   *grpc.ClientConn
	sigClient pb.A2AServiceClient

	nqiConn   *grpc.ClientConn
	nqiClient pb.A2AServiceClient

	mu        sync.Mutex
	contextID string
}

func NewADKService(signallingAddr, nextQuestionAddr string) (ADKServiceInterface, error) {
	sigConn, err := grpc.NewClient(
		signallingAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial signalling %s: %w", signallingAddr, err)
	}

	nqiConn, err := grpc.NewClient(
		nextQuestionAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		sigConn.Close()
		return nil, fmt.Errorf("grpc dial next-question %s: %w", nextQuestionAddr, err)
	}

	svc := &ADKService{
		sigConn:   sigConn,
		sigClient: pb.NewA2AServiceClient(sigConn),
		nqiConn:   nqiConn,
		nqiClient: pb.NewA2AServiceClient(nqiConn),
		contextID: newContextID(),
	}
	log.Printf("[adk] connected — signalling=%s  next-question=%s", signallingAddr, nextQuestionAddr)
	return svc, nil
}

func (s *ADKService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextID = newContextID()
	log.Printf("[adk] reset → contextId=%s", s.contextID)
}

// ── Signal Detector ──────────────────────────────────────────────────────────

func (s *ADKService) ExtractSignalsStream(speaker, transcript string, timestampMs int64) (*SignalResult, error) {
	ts := timeutils.MsToSRT(timestampMs)
	payload := fmt.Sprintf("%s|%s|%s", speaker, ts, transcript)

	s.mu.Lock()
	ctxID := s.contextID
	s.mu.Unlock()

	raw, err := s.streamCall(s.sigClient, ctxID, payload)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	// The signal detector may return an enriched payload that includes NQI fields:
	// {"type":"answer","text":"...","timestamp":"...","next_question":"...","nqi_rationale":"..."}
	var sig struct {
		Type         string `json:"type"`
		Text         string `json:"text"`
		Timestamp    string `json:"timestamp"`
		NextQuestion string `json:"next_question"`
		NQIRationale string `json:"nqi_rationale"`
	}
	if err := json.Unmarshal([]byte(raw), &sig); err != nil {
		log.Printf("[adk] non-JSON signal response: %q", raw)
		return nil, nil
	}
	if sig.Type == "" || sig.Text == "" {
		return nil, nil
	}

	sigTs := sig.Timestamp
	if sigTs == "" {
		sigTs = ts
	}
	return &SignalResult{
		Timestamp:    sigTs,
		Signal:       sig.Type,
		Text:         sig.Text,
		SigLine:      fmt.Sprintf("[%s] [%s] %s", sigTs, strings.ToUpper(sig.Type), sig.Text),
		NextQuestion: sig.NextQuestion,
		NQIRationale: sig.NQIRationale,
	}, nil
}

// ── Next Question Inferrer — AUTO ────────────────────────────────────────────

func (s *ADKService) InferNextQuestion(question, answer string) (*NextQuestionResult, error) {
	inner, err := json.Marshal(map[string]string{
		"question": question,
		"answer":   answer,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal Q/A payload: %w", err)
	}
	payload := fmt.Sprintf("AUTO|%s", string(inner))

	s.mu.Lock()
	nqiCtxID := fmt.Sprintf("nqi-%s", s.contextID)
	s.mu.Unlock()

	return s.callNQI(nqiCtxID, payload)
}

// ── Next Question Inferrer — MANUAL ─────────────────────────────────────────

func (s *ADKService) InferNextQuestionManual(prompt, transcriptSnippet string) (*NextQuestionResult, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt must not be empty")
	}
	payload := fmt.Sprintf("MANUAL|%s|%s", prompt, transcriptSnippet)

	manualCtxID := fmt.Sprintf("nqi-manual-%d", timeutils.NowMs())
	return s.callNQI(manualCtxID, payload)
}

// ── Shared helpers ───────────────────────────────────────────────────────────

func (s *ADKService) callNQI(ctxID, payload string) (*NextQuestionResult, error) {
	raw, err := s.streamCall(s.nqiClient, ctxID, payload)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	var result NextQuestionResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		log.Printf("[adk] non-JSON NQI response: %q", raw)
		return nil, nil
	}
	if result.NextQuestion == "" {
		return nil, nil
	}

	log.Printf("[adk] NQI ctx=%s next_question=%q", ctxID, result.NextQuestion)
	return &result, nil
}

func (s *ADKService) streamCall(client pb.A2AServiceClient, ctxID, payload string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.SendMessageRequest{
		Request: &pb.Message{
			Role:      pb.Role_ROLE_USER,
			ContextId: ctxID,
			MessageId: newMessageID(),
			Parts:     []*pb.Part{{Part: &pb.Part_Text{Text: payload}}},
		},
	}

	stream, err := client.SendStreamingMessage(ctx, req)
	if err != nil {
		return "", fmt.Errorf("SendStreamingMessage: %w", err)
	}

	var sb strings.Builder
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("stream recv: %w", err)
		}
		switch r := resp.Payload.(type) {
		case *pb.StreamResponse_Msg:
			sb.WriteString(partsText(r.Msg))
		case *pb.StreamResponse_StatusUpdate:
			if r.StatusUpdate != nil && r.StatusUpdate.Status != nil {
				if isTerminal(r.StatusUpdate.Status.State) {
					goto done
				}
			}
		}
	}
done:
	raw := strings.TrimSpace(sb.String())
	log.Printf("[adk] ctx=%s raw=%q", ctxID, raw)
	return raw, nil
}

func (s *ADKService) Close() error {
	sigErr := s.sigConn.Close()
	nqiErr := s.nqiConn.Close()
	if sigErr != nil {
		return sigErr
	}
	return nqiErr
}

func isTerminal(state pb.TaskState) bool {
	return state == pb.TaskState_TASK_STATE_COMPLETED ||
		state == pb.TaskState_TASK_STATE_FAILED ||
		state == pb.TaskState_TASK_STATE_CANCELLED ||
		state == pb.TaskState_TASK_STATE_REJECTED
}

func partsText(msg *pb.Message) string {
	if msg == nil {
		return ""
	}
	var out strings.Builder
	for _, part := range msg.Parts {
		if tp, ok := part.Part.(*pb.Part_Text); ok {
			out.WriteString(tp.Text)
		}
	}
	return out.String()
}

func newContextID() string { return fmt.Sprintf("ctx-%d", timeutils.NowMs()) }
func newMessageID() string { return fmt.Sprintf("msg-%d", timeutils.NowMs()) }
