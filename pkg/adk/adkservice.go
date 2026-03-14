package adk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"tal_assistant/pkg/timeutils"
)

// NOTE: use :generateContent (plain JSON), NOT :streamGenerateContent (SSE)
const geminiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"

const geminiSystemPrompt = `You are a signal detector for a live conversation transcript.
You receive text from a live conversation (e.g. a job interview) speaker by speaker.
Your ONLY job is to detect conversation structure and emit these signals:
  [QUESTION_START] [QUESTION_END] [ANSWER_START] [ANSWER_END]
Rules:
- Output ONLY signal tags, nothing else.
- You may emit multiple signals if needed (e.g. [QUESTION_END][ANSWER_START]).
- If nothing significant is detected, respond with exactly: NONE
- Never add any other text.`

type SignalResult struct {
	Timestamp string
	Signal    string
	SigLine   string
}

type ADKServiceInterface interface {
	SendToGemini(speaker, transcript string, timestampMs int64) (*SignalResult, error)
	Reset()
}

type ADKService struct {
	geminiKey           string
	conversationHistory []map[string]interface{}
}

func NewADKService(geminiKey string) ADKServiceInterface {
	return &ADKService{geminiKey: geminiKey}
}

func (a *ADKService) Reset() {
	a.conversationHistory = nil
}

func (a *ADKService) SendToGemini(speaker, transcript string, timestampMs int64) (*SignalResult, error) {
	if a.geminiKey == "" {
		return nil, fmt.Errorf("gemini key not set")
	}

	userMsg := fmt.Sprintf("[%s @ %s]: %s", speaker, timeutils.MsToSRT(timestampMs), transcript)
	a.conversationHistory = append(a.conversationHistory, map[string]interface{}{
		"role":  "user",
		"parts": []map[string]string{{"text": userMsg}},
	})

	body := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{{"text": geminiSystemPrompt}},
		},
		"contents": a.conversationHistory,
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,
			"maxOutputTokens": 50,
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", geminiURL, a.geminiKey)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, string(rawBody))
	}

	log.Printf("[adk] raw response: %.500s", string(rawBody)) // trim to 500 chars

	// Plain JSON response shape:
	// {"candidates":[{"content":{"parts":[{"text":"[QUESTION_START]"}]}}]}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		log.Printf("[adk] no candidates in response")
		return nil, nil
	}

	raw := strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text)
	signal := strings.TrimSpace(strings.ReplaceAll(raw, "NONE", ""))
	log.Printf("[adk] signal=%q", signal)

	if signal == "" {
		return nil, nil
	}

	a.conversationHistory = append(a.conversationHistory, map[string]interface{}{
		"role":  "model",
		"parts": []map[string]string{{"text": signal}},
	})

	ts := timeutils.MsToSRT(timestampMs)
	return &SignalResult{
		Timestamp: ts,
		Signal:    signal,
		SigLine:   fmt.Sprintf("[%s] %s", ts, signal),
	}, nil
}

var _ ADKServiceInterface = (*ADKService)(nil)

func NewMockADKService() ADKServiceInterface { return &mockADKService{} }

type mockADKService struct{}

func (m *mockADKService) SendToGemini(_, _ string, _ int64) (*SignalResult, error) { return nil, nil }
func (m *mockADKService) Reset()                                                   {}
