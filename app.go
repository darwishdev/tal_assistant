package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	speech "cloud.google.com/go/speech/apiv2"
	"cloud.google.com/go/speech/apiv2/speechpb"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	sampleRate = 16000
	chunkSize  = 6400
)

const geminiStreamURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:streamGenerateContent"

const geminiSystemPrompt = `You are a signal detector for a live conversation transcript.
You receive text from a live conversation (e.g. a job interview) speaker by speaker.
Your ONLY job is to detect conversation structure and emit these signals:
  [QUESTION_START] [QUESTION_END] [ANSWER_START] [ANSWER_END]
Rules:
- Output ONLY signal tags, nothing else.
- You may emit multiple signals if needed (e.g. [QUESTION_END][ANSWER_START]).
- If nothing significant is detected, respond with exactly: NONE
- Never add any other text.`

// ── App struct ─────────────────────────────────────────────────────────────

type App struct {
	ctx                 context.Context
	ffmpegCmd           *exec.Cmd // mic
	ffmpegCmd2          *exec.Cmd // speaker
	stopCh              chan struct{}
	conversationHistory []map[string]interface{}
	geminiKey           string
	projectID           string
	srtLines            []string
	sigLines            []string
	srtIndex            int
	doneChannels        int // counts when both streams finish
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.geminiKey = os.Getenv("GEMINI_API_KEY")
	a.projectID = os.Getenv("GOOGLE_PROJECT_ID")

	homeDir, _ := os.UserHomeDir()
	// Find writable dir for log
	logDir := filepath.Join(homeDir, "Desktop")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		logDir = filepath.Join(homeDir, "OneDrive", "Desktop")
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			logDir = homeDir
		}
	}
	logPath := filepath.Join(logDir, "overlay.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("App started")
}

func (a *App) shutdown(ctx context.Context) {
	a.StopRecording()
}

// ── Wails-bound methods ────────────────────────────────────────────────────

func (a *App) GetConfig() map[string]string {
	return map[string]string{
		"projectID": a.projectID,
		"hasGemini": fmt.Sprintf("%v", a.geminiKey != ""),
	}
}

func (a *App) StartRecording(micDevice, speakerDevice string) string {
	if a.ffmpegCmd != nil {
		return "already recording"
	}
	if a.projectID == "" {
		return "error: GOOGLE_PROJECT_ID not set"
	}

	a.stopCh = make(chan struct{})
	a.conversationHistory = nil
	a.srtLines = []string{}
	a.sigLines = []string{"# Signals — " + time.Now().Format("2006-01-02 15:04:05"), ""}
	a.srtIndex = 1

	// ffmpeg for mic — mono PCM
	micCmd := exec.Command("ffmpeg",
		"-f", "dshow",
		"-i", "audio="+micDevice,
		"-af", "afftdn=nr=40", // <— add this filter
		"-ar", "16000",
		"-ac", "1",
		"-f", "s16le",
		"pipe:1",
	)
	micPipe, err := micCmd.StdoutPipe()
	if err != nil {
		return fmt.Sprintf("error mic pipe: %v", err)
	}
	micCmd.Stderr = nil

	// ffmpeg for speaker — mono PCM
	spkCmd := exec.Command("ffmpeg",
		"-f", "dshow", "-i", "audio="+speakerDevice,
		"-ar", "16000", "-ac", "1", "-f", "s16le", "pipe:1",
	)
	spkPipe, err := spkCmd.StdoutPipe()
	if err != nil {
		return fmt.Sprintf("error spk pipe: %v", err)
	}
	spkCmd.Stderr = nil

	if err := micCmd.Start(); err != nil {
		return fmt.Sprintf("error ffmpeg mic: %v", err)
	}
	if err := spkCmd.Start(); err != nil {
		micCmd.Process.Kill()
		return fmt.Sprintf("error ffmpeg spk: %v", err)
	}

	// store both so StopRecording can kill them
	a.ffmpegCmd = micCmd
	a.ffmpegCmd2 = spkCmd

	// two independent speech streams
	go a.runSpeechStreamForChannel(micPipe, "Mic", 1)
	go a.runSpeechStreamForChannel(spkPipe, "Speaker", 2)

	a.emit("status", "recording")
	return "ok"
}

func (a *App) StopRecording() {
	if a.ffmpegCmd != nil {
		a.ffmpegCmd.Process.Kill()
		a.ffmpegCmd = nil
	}
	if a.ffmpegCmd2 != nil {
		a.ffmpegCmd2.Process.Kill()
		a.ffmpegCmd2 = nil
	}
	if a.stopCh != nil {
		select {
		case <-a.stopCh:
		default:
			close(a.stopCh)
		}
	}
	a.emit("status", "stopped")
}

func (a *App) Minimize() {
	runtime.WindowMinimise(a.ctx)
}

func (a *App) Close() {
	a.StopRecording()
	runtime.Quit(a.ctx)
}

func (a *App) SaveFiles(srtContent, signalsContent string) string {
	homeDir, _ := os.UserHomeDir()
	ts := time.Now().Format("2006-01-02_15-04-05")

	srtPath := filepath.Join(homeDir, "transcription_"+ts+".srt")
	sigPath := filepath.Join(homeDir, "signals_"+ts+".txt")

	log.Printf("Saving to: %s", srtPath)

	if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
		return fmt.Sprintf("error writing srt (%s): %v", srtPath, err)
	}
	if err := os.WriteFile(sigPath, []byte(signalsContent), 0644); err != nil {
		return fmt.Sprintf("error writing signals (%s): %v", sigPath, err)
	}
	return fmt.Sprintf("saved: %s", srtPath)
}

// ── Internal ───────────────────────────────────────────────────────────────

func (a *App) emit(event string, data interface{}) {
	runtime.EventsEmit(a.ctx, event, data)
}

func (a *App) logAndEmitError(msg string) {
	log.Println("ERROR:", msg)
	a.emit("error", msg)
}

// ── Google Speech ──────────────────────────────────────────────────────────

func (a *App) runSpeechStreamForChannel(audio io.Reader, label string, channelNum int32) {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()

	go func() {
		select {
		case <-a.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	client, err := speech.NewClient(ctx)
	if err != nil {
		a.logAndEmitError(fmt.Sprintf("Speech client: %v", err))
		return
	}
	defer client.Close()

	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		a.logAndEmitError(fmt.Sprintf("StreamingRecognize: %v", err))
		return
	}

	recognizer := fmt.Sprintf("projects/%s/locations/global/recognizers/_", a.projectID)

	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		Recognizer: recognizer,
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					DecodingConfig: &speechpb.RecognitionConfig_ExplicitDecodingConfig{
						ExplicitDecodingConfig: &speechpb.ExplicitDecodingConfig{
							Encoding:          speechpb.ExplicitDecodingConfig_LINEAR16,
							SampleRateHertz:   sampleRate,
							AudioChannelCount: 1,
						},
					},
					LanguageCodes: []string{"en-US"},
					Model:         "long",
					Features: &speechpb.RecognitionFeatures{
						EnableAutomaticPunctuation: true,
						EnableWordTimeOffsets:      true,
					},
				},
				StreamingFeatures: &speechpb.StreamingRecognitionFeatures{
					InterimResults: true,
				},
			},
		},
	}); err != nil {
		a.logAndEmitError(fmt.Sprintf("send config: %v", err))
		return
	}

	a.emit("status", "recording")

	go func() {
		buf := make([]byte, chunkSize)
		for {
			n, readErr := io.ReadFull(audio, buf)
			if n > 0 {
				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					Recognizer: recognizer,
					StreamingRequest: &speechpb.StreamingRecognizeRequest_Audio{
						Audio: buf[:n],
					},
				}); err != nil {
					return
				}
			}
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				break
			}
			if readErr != nil {
				break
			}
		}
		stream.CloseSend()
	}()

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if ctx.Err() == nil {
				a.logAndEmitError(fmt.Sprintf("recv: %v", err))
			}
			break
		}

		for _, result := range resp.Results {
			if len(result.Alternatives) == 0 {
				continue
			}
			alt := result.Alternatives[0]

			if result.IsFinal {
				var startMs, endMs int64
				if len(alt.Words) > 0 {
					startMs = alt.Words[0].StartOffset.AsDuration().Milliseconds()
					endMs = alt.Words[len(alt.Words)-1].EndOffset.AsDuration().Milliseconds()
				} else {
					endMs = result.ResultEndOffset.AsDuration().Milliseconds()
				}

				a.emit("transcript", map[string]interface{}{
					"label":   label,
					"text":    alt.Transcript,
					"startMs": startMs,
					"endMs":   endMs,
					"isFinal": true,
				})

				// Accumulate SRT in memory
				a.srtLines = append(a.srtLines,
					fmt.Sprintf("%d", a.srtIndex),
					fmt.Sprintf("%s --> %s", msToSRT(startMs), msToSRT(endMs)),
					fmt.Sprintf("[%s] %s", label, alt.Transcript),
					"",
				)
				a.srtIndex++

				if channelNum == 2 {
					go a.sendToGemini(label, alt.Transcript, startMs)
				}
			} else {
				a.emit("transcript", map[string]interface{}{
					"label":   label,
					"text":    alt.Transcript,
					"isFinal": false,
				})
			}
		}
	}

	a.emit("status", "stopped")

	// Auto-save on stop
	if len(a.srtLines) > 0 {
		result := a.SaveFiles(
			strings.Join(a.srtLines, "\n"),
			strings.Join(a.sigLines, "\n"),
		)
		log.Println("Auto-save:", result)
		a.emit("saved", result)
	}
}

// ── Gemini ─────────────────────────────────────────────────────────────────

func (a *App) sendToGemini(speaker, transcript string, timestampMs int64) {
	userMsg := fmt.Sprintf("[%s @ %s]: %s", speaker, msToSRT(timestampMs), transcript)

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

	bodyBytes, _ := json.Marshal(body)
	url := fmt.Sprintf("%s?key=%s&alt=sse", geminiStreamURL, a.geminiKey)

	resp, err := http.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		a.logAndEmitError(fmt.Sprintf("Gemini: %v", err))
		return
	}
	defer resp.Body.Close()

	var fullText strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if t := extractGeminiText(chunk); t != "" {
			fullText.WriteString(t)
		}
	}

	result := strings.TrimSpace(strings.ReplaceAll(fullText.String(), "NONE", ""))
	if result == "" {
		return
	}

	a.conversationHistory = append(a.conversationHistory, map[string]interface{}{
		"role":  "model",
		"parts": []map[string]string{{"text": result}},
	})

	a.sigLines = append(a.sigLines, fmt.Sprintf("[%s] %s", msToSRT(timestampMs), result))
	a.emit("signal", map[string]string{
		"timestamp": msToSRT(timestampMs),
		"signal":    result,
	})
}

// ── Helpers ────────────────────────────────────────────────────────────────

func msToSRT(ms int64) string {
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	s := (ms % 60000) / 1000
	f := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, f)
}

func extractGeminiText(result map[string]interface{}) (text string) {
	defer func() { recover() }()
	c := result["candidates"].([]interface{})[0].(map[string]interface{})
	p := c["content"].(map[string]interface{})["parts"].([]interface{})[0].(map[string]interface{})
	return p["text"].(string)
}
