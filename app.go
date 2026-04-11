package main

import (
	"context"
	"encoding/json"
	"fmt"

	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"tal_assistant/config"
	"tal_assistant/pkg/adk"
	"tal_assistant/pkg/adkutils"
	"tal_assistant/pkg/ffmpeg"
	redispkg "tal_assistant/pkg/redis"
	"tal_assistant/pkg/stt"
	"tal_assistant/pkg/timeutils"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ── App struct ─────────────────────────────────────────────────────────────

type sigJob struct {
	speaker string
	text    string
	startMs int64
}

type App struct {
	ctx              context.Context
	ffmpegService    *ffmpeg.FFMPEGService
	redisSub         *redispkg.RedisSubscriber
	redisCancel      context.CancelFunc
	currentVideoPath string
	sigQueue         chan sigJob
	redisSubscriber  *redispkg.OrchestrationSubscriber
	redisPublisher   *redispkg.RedisPublisher
	redisCache       redispkg.RedisCacheInterface
	sttService       stt.STTServiceInterface
	adkService       adk.ADKServiceInterface
	ffmpegCmd        *exec.Cmd
	ffmpegCmd2       *exec.Cmd
	stopCh           chan struct{}

	sessions    *adk.InterviewSessions
	interviewID string
	userID      string

	geminiKey string
	projectID string
	srtLines  []string
	sigLines  []string
	srtIndex  int
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg := config.Load()
	a.geminiKey = cfg.GoogleAPIKey
	a.projectID = cfg.GoogleProjectID
	fmt.Println("project id is ", a.projectID)

	homeDir, _ := os.UserHomeDir()
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
	if err != nil {
		log.Printf("STT service error: %v", err)
	}
	a.sttService = sttService

	a.ffmpegService = ffmpeg.NewFFMPEGService()

	adkService, err := adk.NewADKService(ctx, a.geminiKey)
	if err != nil {
		log.Printf("NewADKService: %v", err)
	}
	a.adkService = adkService

	publisher := redispkg.NewRedisPublisher()
	a.redisPublisher = publisher

	redisCache := redispkg.NewRedisCacheClient()
	a.redisCache = redisCache

	subscriber := redispkg.NewOrchestrationSubscriber(adkService, publisher, redisCache, a.emit)
	a.redisSubscriber = subscriber

	redisCtx, redisCancel := context.WithCancel(context.Background())
	a.redisCancel = redisCancel
	go a.redisSubscriber.Run(redisCtx)

	log.Println("App started")
}

func (a *App) shutdown(ctx context.Context) {
	if a.redisCancel != nil {
		a.redisCancel()
	}
	if a.redisSub != nil {
		a.redisSub.Close()
	}
	a.StopRecording()
}

// ── Session init ───────────────────────────────────────────────────────────
// StartSession initialises ADK sessions for all agents and seeds Redis with
// the question bank.  Call this before StartRecording.

func (a *App) StartSession(userID string, questionBank []adkutils.QuestionBankQuestion) string {
	a.userID = userID
	a.interviewID = fmt.Sprintf("%s_%d", userID, time.Now().UnixMilli())

	sessions, err := a.adkService.StartSession(a.ctx, userID, questionBank)
	if err != nil {
		return fmt.Sprintf("error starting ADK session: %v", err)
	}
	a.sessions = &sessions

	if err := a.redisCache.SaveQuestionBank(a.ctx, a.interviewID, questionBank); err != nil {
		log.Printf("SaveQuestionBank: %v", err)
	}
	if err := a.redisCache.InitInterviewSummary(a.ctx, a.interviewID, questionBank); err != nil {
		log.Printf("InitInterviewSummary: %v", err)
	}

	log.Printf("Session started: interview=%s user=%s signaling=%s",
		a.interviewID, userID, sessions.SignalingAgentSessionID)
	return "ok"
}

// ── Signal queue worker ────────────────────────────────────────────────────
// Processes STT results one at a time — no concurrent calls to the signal
// detector so the Redis session state is never raced.

func (a *App) processSigQueue() {
	var qandaBuf strings.Builder
	for job := range a.sigQueue {
		if a.sessions == nil {
			a.logAndEmitError("processSigQueue: no active session — call StartSession first")
			continue
		}

		// Stream the signaling agent and emit each chunk to the UI.
		var sigBuf strings.Builder
		for chunk, err := range a.adkService.SignalingAgentRun(adkutils.AgentRunRequest{
			Ctx:       a.ctx,
			SessionID: a.sessions.SignalingAgentSessionID,
			UserID:    a.userID,
			Prompt:    job.speaker + ": " + job.text,
		}) {
			if err != nil {
				a.logAndEmitError(fmt.Sprintf("signal extraction: %v", err))
				break
			}
			a.emit("signal_chunk_detected", chunk)
			sigBuf.WriteString(chunk)
		}

		signal := strings.TrimSpace(sigBuf.String())
		log.Printf("[sig-queue] speaker=%s signal=%q", job.speaker, signal)

		if signal == "" || signal == "UNCLEAR" {
			continue
		}

		// Accumulate Q&A context: reset buffer on a new question signal.
		if strings.HasPrefix(signal, "Q:") {
			qandaBuf.Reset()
		}
		qandaBuf.WriteString(signal)
		qandaBuf.WriteString("\n")

		// Publish to the orchestration pipeline via Redis.
		if err := a.redisPublisher.PublishSignalDetected(a.ctx, redispkg.SignalDetectedEvent{
			InterviewID:        a.interviewID,
			UserID:             a.userID,
			TranscriptLine:     job.text,
			Signal:             signal,
			QAndA:              qandaBuf.String(),
			SignalingSessionID: a.sessions.SignalingAgentSessionID,
			MapperSessionID:    a.sessions.SignalingAgentMapperSessionID,
			IndicatorSessionID: a.sessions.NextQuestionIndicatorSessionID,
			ExtenderSessionID:  a.sessions.NextQuestionExtenderSessionID,
		}); err != nil {
			a.logAndEmitError(fmt.Sprintf("publish signal detected: %v", err))
		}
	}
	log.Println("[sig-queue] worker stopped")
}

// ── Wails-bound methods ────────────────────────────────────────────────────
func (a *App) StartRecording(micDevice, speakerDevice, screenDevice string) string {
	if a.ffmpegCmd != nil {
		return "already recording"
	}
	if a.projectID == "" {
		return "error: GOOGLE_PROJECT_ID not set"
	}

	a.stopCh = make(chan struct{})
	a.srtLines = []string{}
	a.sigLines = []string{"# Signals — " + time.Now().Format("2006-01-02 15:04:05"), ""}
	a.srtIndex = 1

	a.sigQueue = make(chan sigJob, 50)
	go a.processSigQueue()

	// ── Audio pipe for STT ────────────────────────────────────────────────
	audioPipe, err := a.ffmpegService.Start(micDevice, speakerDevice)
	if err != nil {
		return fmt.Sprintf("error audio pipe: %v", err)
	}

	// ── Screen recording (optional) ───────────────────────────────────────
	outputDir := a.sessionOutputDir()

	var screen *ffmpeg.ScreenSource
	if screenDevice != "" {
		// find the matching ScreenSource from the device list
		screens, err := a.ffmpegService.ScreenDeviceList(a.ctx)
		if err == nil {
			for i, s := range screens {
				if s.ID == screenDevice || s.Name == screenDevice {
					screen = &screens[i]
					break
				}
			}
		}
		// screen == nil here means "full desktop" — StartScreenRecording handles that
	}

	if screenDevice != "" || screen != nil {
		videoPath, err := a.ffmpegService.StartScreenRecording(screen, micDevice, speakerDevice, outputDir)
		if err != nil {
			log.Printf("screen recording failed to start (non-fatal): %v", err)
		} else {
			a.currentVideoPath = videoPath
			log.Printf("screen recording started → %s", videoPath)
		}
	}

	go a.runSpeechStream(audioPipe)
	a.emit("status", "recording")
	return "ok"
}

func (a *App) StopRecording() {
	a.ffmpegService.Stop()
	a.ffmpegService.StopScreenRecording()

	if a.stopCh != nil {
		select {
		case <-a.stopCh:
		default:
			close(a.stopCh)
		}
		a.stopCh = nil
	}

	if a.sigQueue != nil {
		close(a.sigQueue)
		a.sigQueue = nil
	}

	a.emit("status", "stopped")
}
func (a *App) Minimize() { runtime.WindowMinimise(a.ctx) }

func (a *App) Close() {
	a.StopRecording()
	runtime.Quit(a.ctx)
}

// sessionOutputDir returns (and creates) the directory where all session
// files — srt, signals, video, meta — will be saved.
func (a *App) sessionOutputDir() string {
	homeDir, _ := os.UserHomeDir()
	return homeDir
}
func (a *App) SaveFiles(srtContent, signalsContent string) string {
	outputDir := a.sessionOutputDir()
	ts := time.Now().Format("2006-01-02_15-04-05")

	srtPath := filepath.Join(outputDir, "transcription_"+ts+".srt")
	sigPath := filepath.Join(outputDir, "signals_"+ts+".txt")
	metaPath := filepath.Join(outputDir, "session_"+ts+".json")

	log.Printf("Saving to: %s", srtPath)

	if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
		return fmt.Sprintf("error writing srt (%s): %v", srtPath, err)
	}
	if err := os.WriteFile(sigPath, []byte(signalsContent), 0644); err != nil {
		return fmt.Sprintf("error writing signals (%s): %v", sigPath, err)
	}

	meta := map[string]string{
		"recorded_at": ts,
		"transcript":  srtPath,
		"signals":     sigPath,
		"video":       a.currentVideoPath,
	}
	if metaBytes, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(metaPath, metaBytes, 0644)
	}

	a.currentVideoPath = ""
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
			// Push to queue — processed sequentially, no race on session state
			if a.sigQueue != nil {
				a.sigQueue <- sigJob{
					speaker: label,
					text:    res.Text,
					startMs: res.StartMs,
				}
			}

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

	if len(a.srtLines) > 0 {
		result := a.SaveFiles(
			strings.Join(a.srtLines, "\n"),
			strings.Join(a.sigLines, "\n"),
		)
		log.Println("Auto-save:", result)
		a.emit("saved", result)
	}
}

// ── Audio / screen devices ─────────────────────────────────────────────────

type DeviceSource struct {
	ID   string
	Name string
}

type ListSourcesResonse struct {
	Mics     []DeviceSource
	Speakers []DeviceSource
	Screens  []ffmpeg.ScreenSource
}

func (a *App) ListAudioDevices() (*ListSourcesResonse, error) {
	audioDevices, err := a.ffmpegService.AudioDeviceList(a.ctx)
	if err != nil {
		return nil, err
	}

	var mics, speakers []DeviceSource
	for _, dev := range audioDevices {
		src := DeviceSource{ID: dev.ID, Name: dev.Name}
		if dev.Type == "mic" {
			mics = append(mics, src)
		} else {
			speakers = append(speakers, src)
		}
	}

	screenDevices, err := a.ffmpegService.ScreenDeviceList(a.ctx)
	if err != nil {
		return nil, err
	}

	var screens []ffmpeg.ScreenSource
	for _, screen := range screenDevices {
		screens = append(screens, ffmpeg.ScreenSource{
			ID: screen.ID, Name: screen.Name,
			Screenshot: screen.Screenshot})
	}

	return &ListSourcesResonse{Mics: mics, Speakers: speakers, Screens: screens}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
