package main

import (
	"context"

	"fmt"
	"io"
	"log"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"tal_assistant/pkg/adk"
	"tal_assistant/pkg/ffmpeg"
	"tal_assistant/pkg/stt"
	"tal_assistant/pkg/timeutils"

	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ── App struct ─────────────────────────────────────────────────────────────

type App struct {
	ctx                 context.Context
	ffmpegService       *ffmpeg.FFMPEGService
	sttService          stt.STTServiceInterface
	adkService          adk.ADKServiceInterface
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
	sttService, err := stt.NewSTTService(a.projectID)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}
	a.sttService = sttService
	ffmpegService := ffmpeg.NewFFMPEGService()
	a.ffmpegService = ffmpegService
	fmt.Println("geminig", a.geminiKey)
	a.adkService = adk.NewADKService(a.geminiKey)
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

	audioPipe, err := a.ffmpegService.Start(micDevice, speakerDevice)
	if err != nil {
		return fmt.Sprintf("error audio pipe: %v", err)
	}
	go a.runSpeechStream(audioPipe)
	a.adkService.Reset()
	a.emit("status", "recording")
	return "ok"
}

func (a *App) StopRecording() {
	a.ffmpegService.Stop()
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
func (a *App) runSpeechStream(audio io.Reader) {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()

	go func() {
		select {
		case <-a.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	results, err := a.sttService.StreamDiarized(ctx, audio)
	if err != nil {
		a.logAndEmitError(fmt.Sprintf("stt stream: %v", err))
		return
	}

	a.emit("status", "recording")

	for res := range results {

		label := res.SpeakerTag

		a.emit("transcript", map[string]interface{}{
			"label":   label,
			"text":    res.Text,
			"startMs": res.StartMs,
			"endMs":   res.EndMs,
			"isFinal": res.IsFinal,
		})

		if res.IsFinal {
			go func(speaker, text string, startMs int64) {
				fmt.Println("text is ", text)
				sig, err := a.adkService.SendToGemini(speaker, text, startMs)
				if err != nil {
					a.logAndEmitError(fmt.Sprintf("Gemini: %v", err))
					return
				}
				fmt.Println("signnal is", sig)
				fmt.Println("err is", err)
				if sig != nil {
					a.sigLines = append(a.sigLines, sig.SigLine)
					a.emit("signal", map[string]string{
						"timestamp": sig.Timestamp,
						"signal":    sig.Signal,
					})
				}
			}(label, res.Text, res.StartMs)
			a.srtLines = append(a.srtLines,
				fmt.Sprintf("%d", a.srtIndex),
				fmt.Sprintf("%s --> %s",
					timeutils.MsToSRT(res.StartMs),
					timeutils.MsToSRT(res.EndMs)),
				fmt.Sprintf("[%s] %s", label, res.Text),
				"",
			)

			a.srtIndex++

		}
	}

	a.emit("status", "stopped")

	if len(a.srtLines) > 0 {
		result := a.SaveFiles(
			strings.Join(a.srtLines, "\n"),
			strings.Join(a.sigLines, "\n"),
		)

		log.Println("Auto-save:", result)
		a.emit("saved", result)
	}
}
