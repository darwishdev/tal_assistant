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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"tal_assistant/pkg/timeutils"

	pb "github.com/a2aproject/a2a-go/a2apb"
)

// SignalResult now carries the full detected Q or A
type SignalResult struct {
	Timestamp string // when the segment started
	Signal    string // "question" or "answer"
	Text      string // the full detected question or answer text
	SigLine   string // formatted log line
}

type ADKServiceInterface interface {
	ExtractSignalsStream(speaker, transcript string, timestampMs int64) (*SignalResult, error)
	Reset()
	Close() error
}

type ADKService struct {
	conn      *grpc.ClientConn
	client    pb.A2AServiceClient
	mu        sync.Mutex
	contextID string
}

func NewADKService(addr string) (ADKServiceInterface, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	svc := &ADKService{
		conn:      conn,
		client:    pb.NewA2AServiceClient(conn),
		contextID: newContextID(),
	}
	log.Printf("[adk] connected to %s", addr)
	return svc, nil
}

func (s *ADKService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextID = newContextID()
	log.Printf("[adk] reset → contextId=%s", s.contextID)
}

func (s *ADKService) ExtractSignalsStream(speaker, transcript string, timestampMs int64) (*SignalResult, error) {
	ts := timeutils.MsToSRT(timestampMs)
	payload := fmt.Sprintf("%s|%s|%s", speaker, ts, transcript)

	s.mu.Lock()
	ctxID := s.contextID
	s.mu.Unlock()

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

	stream, err := s.client.SendStreamingMessage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("SendStreamingMessage: %w", err)
	}

	var raw string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("stream recv: %w", err)
		}
		switch r := resp.Payload.(type) {
		case *pb.StreamResponse_Msg:
			raw += partsText(r.Msg)
		case *pb.StreamResponse_StatusUpdate:
			if r.StatusUpdate != nil && r.StatusUpdate.Status != nil {
				if isTerminal(r.StatusUpdate.Status.State) {
					goto done
				}
			}
		}
	}

done:
	raw = strings.TrimSpace(raw)
	log.Printf("[adk] ctx=%s raw=%q", ctxID, raw)

	if raw == "" {
		return nil, nil // silent — model still observing
	}

	// Parse JSON signal from Python
	var sig struct {
		Type      string `json:"type"`
		Text      string `json:"text"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(raw), &sig); err != nil {
		log.Printf("[adk] non-JSON response: %q", raw)
		return nil, nil
	}

	if sig.Type == "" || sig.Text == "" {
		return nil, nil
	}

	// Use the timestamp from the signal if provided, else fall back to current
	sigTs := sig.Timestamp
	if sigTs == "" {
		sigTs = ts
	}

	return &SignalResult{
		Timestamp: sigTs,
		Signal:    sig.Type,
		Text:      sig.Text,
		SigLine:   fmt.Sprintf("[%s] [%s] %s", sigTs, strings.ToUpper(sig.Type), sig.Text),
	}, nil
}

func (s *ADKService) Close() error { return s.conn.Close() }

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
	out := ""
	for _, part := range msg.Parts {
		if tp, ok := part.Part.(*pb.Part_Text); ok {
			out += tp.Text
		}
	}
	return out
}

func newContextID() string { return fmt.Sprintf("ctx-%d", timeutils.NowMs()) }
func newMessageID() string { return fmt.Sprintf("msg-%d", timeutils.NowMs()) }
