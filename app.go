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
	"tal_assistant/pkg/adk/questionbankgenerator"
	"tal_assistant/pkg/adkutils"
	"tal_assistant/pkg/atsclient"
	"tal_assistant/pkg/recording"
	redispkg "tal_assistant/pkg/redis"
	"tal_assistant/pkg/stt"
	"tal_assistant/pkg/timeutils"
	"tal_assistant/pkg/workableclient"
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
	ffmpegService    *recording.RecordingService
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

	sessions            *adk.InterviewSessions
	interviewID         string
	userID              string
	initialQuestionText string // first question text, emitted to UI when recording starts

	atsClient      atsclient.ATSClientInterface
	workableClient workableclient.ClientInterface

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
	fmt.Printf("api key is %s", cfg.GoogleAPIKey)
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

	// Initialize STT service with credentials priority:
	// 1. External file (if GOOGLE_CREDENTIALS_PATH is set and file exists)
	// 2. Embedded credentials (compiled into the binary) - skipped in DEV_MODE
	// 3. Application Default Credentials (fallback for development)
	var sttService stt.STTServiceInterface

	if cfg.GoogleCredentialsPath != "" {
		// Try external credentials file first
		if _, err := os.Stat(cfg.GoogleCredentialsPath); err == nil {
			sttService, err = stt.NewSTTService(a.projectID, cfg.GoogleCredentialsPath)
			if err != nil {
				log.Printf("[startup] WARNING: Failed to use external credentials file: %v", err)
			} else {
				log.Printf("[startup] STT service initialized with external credentials: %s", cfg.GoogleCredentialsPath)
			}
		} else {
			log.Printf("[startup] External credentials file not found: %s", cfg.GoogleCredentialsPath)
		}
	}

	// If external credentials failed or not provided, try embedded credentials (unless in dev mode)
	if sttService == nil && !cfg.DevMode {
		embeddedCreds := config.GetEmbeddedCredentials()
		if embeddedCreds != nil {
			sttService, err = stt.NewSTTServiceWithCredentials(a.projectID, embeddedCreds)
			if err != nil {
				log.Printf("[startup] WARNING: Failed to use embedded credentials: %v", err)
			} else {
				log.Printf("[startup] STT service initialized with embedded credentials")
			}
		}
	} else if cfg.DevMode {
		log.Printf("[startup] DEV_MODE enabled - skipping embedded credentials, will use Application Default Credentials")
	}

	// Final fallback: try Application Default Credentials
	if sttService == nil {
		sttService, err = stt.NewSTTService(a.projectID, "")
		if err != nil {
			log.Printf("[startup] WARNING: STT service initialization failed: %v", err)
			log.Printf("[startup] Transcription will not be available. Please check Google Cloud credentials.")
			a.sttService = nil
		} else {
			log.Printf("[startup] STT service initialized with Application Default Credentials")
		}
	}

	a.sttService = sttService

	a.ffmpegService = recording.NewRecordingService()

	adkService, err := adk.NewADKService(ctx, a.geminiKey)
	if err != nil {
		log.Printf("NewADKService: %v", err)
	}
	a.adkService = adkService

	atsClient, err := atsclient.NewATSClient(cfg.ATSBaseURL)
	if err != nil {
		log.Printf("ATSClient init error: %v", err)
	}
	a.atsClient = atsClient

	if cfg.WorkableSubdomain != "" && cfg.WorkableToken != "" {
		wc, err := workableclient.New(cfg.WorkableSubdomain, cfg.WorkableToken)
		if err != nil {
			log.Printf("WorkableClient init error: %v", err)
		} else {
			a.workableClient = wc
		}
	} else {
		log.Println("warning: WORKABLE_SUBDOMAIN or WORKABLE_TOKEN not set, workable client unavailable")
	}

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

	log.Printf("[session] initialising ADK sessions — user=%q interview=%s questions=%d",
		userID, a.interviewID, len(questionBank))

	sessions, err := a.adkService.StartSession(a.ctx, userID, questionBank)
	if err != nil {
		log.Printf("[session] ERROR: ADK StartSession failed: %v", err)
		return fmt.Sprintf("error starting ADK session: %v", err)
	}
	a.sessions = &sessions

	log.Printf("[session] ADK sessions created — signaling=%s mapper=%s nqi=%s nqe=%s judging=%s",
		sessions.SignalingAgentSessionID,
		sessions.SignalingAgentMapperSessionID,
		sessions.NextQuestionIndicatorSessionID,
		sessions.NextQuestionExtenderSessionID,
		sessions.JudgingAgentSessionID,
	)

	if len(questionBank) > 0 {
		if err := a.redisCache.SaveQuestionBank(a.ctx, a.interviewID, questionBank); err != nil {
			log.Printf("[session] WARNING: SaveQuestionBank: %v", err)
		} else {
			log.Printf("[session] question bank saved to Redis — interview=%s questions=%d", a.interviewID, len(questionBank))
		}
	} else {
		log.Printf("[session] WARNING: no question bank provided — mapper will not be able to match signals")
	}

	if err := a.redisCache.InitInterviewSummary(a.ctx, a.interviewID, questionBank); err != nil {
		log.Printf("[session] WARNING: InitInterviewSummary: %v", err)
	}

	log.Printf("[session] ready — interview=%s user=%s", a.interviewID, userID)
	return "ok"
}

// ── cleanTranscription ─────────────────────────────────────────────────────
// Removes punctuation marks (commas, dots) from transcription text before
// sending to the signaling agent.

func cleanTranscription(text string) string {
	replacer := strings.NewReplacer(",", "", ".", "")
	return replacer.Replace(text)
}

// ── Signal queue worker ────────────────────────────────────────────────────
// Processes STT results one at a time — no concurrent calls to the signal
// detector so the Redis session state is never raced.

func (a *App) processSigQueue() {
	log.Printf("[sig-queue] worker started — interview=%s", a.interviewID)
	var qandaBuf strings.Builder

	for job := range a.sigQueue {
		if a.sessions == nil {
			a.logAndEmitError("no active session — call StartSession before recording")
			continue
		}

		log.Printf("[sig-queue] processing utterance speaker=%q len=%d", job.speaker, len(job.text))

		// ── Stream signaling agent ────────────────────────────────────────
		cleanedText := cleanTranscription(job.text)
		var sigBuf strings.Builder
		for chunk, err := range a.adkService.SignalingAgentRun(adkutils.AgentRunRequest{
			Ctx:       a.ctx,
			SessionID: a.sessions.SignalingAgentSessionID,
			UserID:    a.userID,
			Prompt:    job.speaker + ": " + cleanedText,
		}) {
			if err != nil {
				a.logAndEmitError(fmt.Sprintf("signaling agent error: %v", err))
				break
			}
			a.emit("signal_chunk_detected", chunk)
			sigBuf.WriteString(chunk)
		}

		signal := strings.TrimSpace(sigBuf.String())

		// ── Deduplication ─────────────────────────────────────────────────
		// The ADK SDK yields incremental chunks then emits the complete text
		// as a final event, causing signal = "content" + "content".
		// Detect exact doubling and strip the duplicate half.
		if n := len(signal); n > 0 && n%2 == 0 {
			if signal[:n/2] == signal[n/2:] {
				signal = signal[:n/2]
				log.Printf("[sig-queue] dedup: stripped duplicate — final len=%d", len(signal))
			}
		}

		// ── Classify & guard ──────────────────────────────────────────────
		switch {
		case signal == "" || signal == "UNCLEAR":
			log.Printf("[sig-queue] speaker=%q → UNCLEAR, skipping", job.speaker)
			continue
		case strings.HasPrefix(signal, "Q:"):
			log.Printf("[sig-queue] speaker=%q → QUESTION: %q", job.speaker, signal[2:])
			qandaBuf.Reset()
		case strings.HasPrefix(signal, "A:"):
			answerText := signal[2:]
			if strings.HasSuffix(strings.TrimSpace(answerText), ";") {
				log.Printf("[sig-queue] speaker=%q → ANSWER COMPLETE: %q", job.speaker, answerText)
			} else {
				log.Printf("[sig-queue] speaker=%q → ANSWER IN PROGRESS: %q", job.speaker, answerText)
			}
		default:
			log.Printf("[sig-queue] speaker=%q → UNRECOGNISED signal=%q, skipping", job.speaker, signal)
			continue
		}

		qandaBuf.WriteString(signal)
		qandaBuf.WriteString("\n")

		// ── Publish to orchestration pipeline ─────────────────────────────
		event := redispkg.SignalDetectedEvent{
			InterviewID:        a.interviewID,
			UserID:             a.userID,
			TranscriptLine:     job.text,
			Signal:             signal,
			QAndA:              qandaBuf.String(),
			SignalingSessionID: a.sessions.SignalingAgentSessionID,
			MapperSessionID:    a.sessions.SignalingAgentMapperSessionID,
			IndicatorSessionID: a.sessions.NextQuestionIndicatorSessionID,
			ExtenderSessionID:  a.sessions.NextQuestionExtenderSessionID,
			JudgingSessionID:   a.sessions.JudgingAgentSessionID,
		}
		if err := a.redisPublisher.PublishSignalDetected(a.ctx, event); err != nil {
			a.logAndEmitError(fmt.Sprintf("publish signal_detected: %v", err))
		} else {
			log.Printf("[sig-queue] → published signal_detected interview=%s signal=%q",
				a.interviewID, signal[:min(40, len(signal))])
		}
	}

	log.Printf("[sig-queue] worker stopped — interview=%s", a.interviewID)
}

// ── Wails-bound methods ────────────────────────────────────────────────────
func (a *App) StartRecording(micDevice, speakerDevice, screenDevice string) string {
	if a.ffmpegCmd != nil {
		log.Println("[recording] already recording — ignoring StartRecording call")
		return "already recording"
	}
	if a.projectID == "" {
		log.Println("[recording] ERROR: GOOGLE_PROJECT_ID not set")
		return "error: GOOGLE_PROJECT_ID not set"
	}
	if a.sessions == nil {
		log.Println("[recording] ERROR: no active session — call ATSBeginSession before StartRecording")
		return "error: no active session — call ATSBeginSession first"
	}

	log.Printf("[recording] starting — interview=%s mic=%q speaker=%q screen=%q",
		a.interviewID, micDevice, speakerDevice, screenDevice)

	a.stopCh = make(chan struct{})
	a.srtLines = []string{}
	a.sigLines = []string{"# Signals — " + time.Now().Format("2006-01-02 15:04:05"), ""}
	a.srtIndex = 1

	a.sigQueue = make(chan sigJob, 50)
	go a.processSigQueue()

	// Push the first question to the UI immediately so the recruiter sees it
	// as soon as the active-session view renders.
	if a.initialQuestionText != "" {
		a.emit("current_question", a.initialQuestionText)
	}

	// ── Audio pipe for STT ────────────────────────────────────────────────
	audioPipe, channels, err := a.ffmpegService.Start(micDevice, speakerDevice)
	if err != nil {
		log.Printf("[recording] ERROR: audio pipe failed: %v", err)
		return fmt.Sprintf("error audio pipe: %v", err)
	}
	log.Printf("[recording] audio pipe open (channels=%d) — STT stream starting", channels)

	// ── 2. STT stream ──────────────────────────────────────────────────────
	if a.sttService == nil {
		a.logAndEmitError("STT service not available - transcription disabled. Check Google Cloud credentials.")
		log.Printf("[recording] WARNING: STT service is nil, skipping transcription")
	} else {
		go a.runSpeechStream(audioPipe, channels)
	}

	// ── Screen recording (optional) ───────────────────────────────────────
	outputDir := a.sessionOutputDir()

	var screen *recording.ScreenSource
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

func (a *App) runSpeechStream(audio io.Reader, channels int) {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()

	go func() {
		select {
		case <-a.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	results, err := a.sttService.StreamDiarized(ctx, audio, channels)
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
	ID         string
	Name       string
	IsDefault  bool
	IsPersonal bool
}

type ListSourcesResonse struct {
	Mics     []DeviceSource
	Speakers []DeviceSource
	Screens  []recording.ScreenSource
}

func (a *App) ListAudioDevices() (*ListSourcesResonse, error) {
	audioDevices, err := a.ffmpegService.AudioDeviceList(a.ctx)
	if err != nil {
		return nil, err
	}

	var mics, speakers []DeviceSource
	for _, dev := range audioDevices {
		src := DeviceSource{
			ID:         dev.ID,
			Name:       dev.Name,
			IsDefault:  dev.IsDefault,
			IsPersonal: dev.IsPersonal,
		}
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

	var screens []recording.ScreenSource
	for _, screen := range screenDevices {
		screens = append(screens, recording.ScreenSource{
			ID: screen.ID, Name: screen.Name,
			Screenshot: screen.Screenshot})
	}

	return &ListSourcesResonse{Mics: mics, Speakers: speakers, Screens: screens}, nil
}

// ── ATS client — Wails-bound methods ──────────────────────────────────────

// ATSLogin authenticates against the ATS and stores the session cookie.
// Returns the AppLoginResponse or an error string.
func (a *App) ATSLogin(username, password string) (*AppLoginResponse, error) {
	atsResp, err := a.atsClient.Login(username, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	result := &AppLoginResponse{
		ATSLogin: atsResp,
	}
	fmt.Println("workable client", a.workableClient, username)
	if a.workableClient != nil {
		opts := workableclient.ListMembersOptions{
			Email: username,
			Limit: 1,
		}

		members, err := a.workableClient.ListMembers(opts)
		fmt.Println("Members ", members)
		if err == nil && len(members) > 0 {
			result.Member = &members[0]
		} else if err != nil {
			log.Printf("warning: failed to fetch workable member for %s: %v", username, err)
		} else {
			log.Printf("warning: no workable member found for %s", username)
		}
	}

	return result, nil
}

// ATSInterviewList returns all interviews visible to the current session.
func (a *App) ATSInterviewList() ([]atsclient.InterviewListItem, error) {
	return a.atsClient.InterviewList()
}

// WorkableInterviewList returns future events for a given member.
func (a *App) WorkableInterviewList(memberID string) ([]workableclient.Event, error) {
	if a.workableClient == nil {
		return nil, fmt.Errorf("workable client not initialized")
	}
	opts := workableclient.ListEventsOptions{
		MemberID: memberID,
	}
	return a.workableClient.ListFutureEvents(opts)
}

// WorkableEventFind fetches a single Workable event by ID along with its full
// job and candidate details.
func (a *App) WorkableEventFind(eventID string) (*workableclient.EventFindResult, error) {
	if a.workableClient == nil {
		return nil, fmt.Errorf("workable client not configured")
	}
	return a.workableClient.EventFind(eventID)
}

// GenerateQuestionBank fetches job+candidate data for the given Workable event,
// runs the question-bank-generator ADK agent, and persists the result in Redis
// keyed by eventID. Returns "ok" on success or an error string.
func (a *App) GenerateQuestionBank(eventID string) string {
	if a.workableClient == nil {
		return "error: workable client not configured"
	}

	result, err := a.workableClient.EventFind(eventID)
	if err != nil {
		return fmt.Sprintf("error: fetch event %s: %v", eventID, err)
	}

	// Build experience text
	var expParts []string
	if result.Candidate != nil {
		for _, ex := range result.Candidate.ExperienceEntries {
			part := ""
			if ex.Title != nil {
				part += *ex.Title
			}
			if ex.Company != nil {
				part += " @ " + *ex.Company
			}
			if ex.StartDate != nil {
				part += " (" + *ex.StartDate
				if ex.EndDate != nil {
					part += " – " + *ex.EndDate
				} else if ex.Current {
					part += " – Present"
				}
				part += ")"
			}
			if ex.Summary != nil {
				part += ": " + *ex.Summary
			}
			if part != "" {
				expParts = append(expParts, part)
			}
		}
	}
	experienceText := strings.Join(expParts, "\n")

	// Build education text
	var eduParts []string
	if result.Candidate != nil {
		for _, ed := range result.Candidate.EducationEntries {
			part := ""
			if ed.Degree != nil {
				part += *ed.Degree
			}
			if ed.FieldOfStudy != nil {
				part += " in " + *ed.FieldOfStudy
			}
			if ed.School != nil {
				part += " at " + *ed.School
			}
			if ed.EndDate != nil {
				part += " (" + *ed.EndDate + ")"
			}
			if part != "" {
				eduParts = append(eduParts, part)
			}
		}
	}
	educationText := strings.Join(eduParts, "\n")

	// Build skills text
	var skillParts []string
	if result.Candidate != nil {
		for _, s := range result.Candidate.Skills {
			switch v := s.(type) {
			case string:
				skillParts = append(skillParts, v)
			case map[string]interface{}:
				if name, ok := v["name"].(string); ok {
					skillParts = append(skillParts, name)
				}
			}
		}
	}
	skillsText := strings.Join(skillParts, ", ")

	candidateName := ""
	candidateSummary := ""
	if result.Candidate != nil {
		candidateName = result.Candidate.Name
		if result.Candidate.Summary != nil {
			candidateSummary = *result.Candidate.Summary
		}
	}

	jobTitle := ""
	jobDescription := ""
	jobRequirements := ""
	if result.Job != nil {
		jobTitle = result.Job.Title
		if result.Job.Description != nil {
			jobDescription = *result.Job.Description
		}
		if result.Job.Requirements != nil {
			jobRequirements = *result.Job.Requirements
		}
	}

	input := questionbankgenerator.QuestionBankGeneratorInput{
		JobTitle:            jobTitle,
		JobDescription:      jobDescription,
		JobRequirements:     jobRequirements,
		CandidateName:       candidateName,
		CandidateSummary:    candidateSummary,
		CandidateExperience: experienceText,
		CandidateEducation:  educationText,
		CandidateSkills:     skillsText,
	}

	state := a.adkService.NewQuestionBankGeneratorState(questionbankgenerator.QuestionBankGeneratorState{
		JobTitle:            jobTitle,
		JobDescription:      jobDescription,
		JobRequirements:     jobRequirements,
		CandidateName:       candidateName,
		CandidateSummary:    candidateSummary,
		CandidateExperience: experienceText,
		CandidateEducation:  educationText,
		CandidateSkills:     skillsText,
	})

	sessionID := fmt.Sprintf("qbgen_%s_%d", eventID, time.Now().UnixMilli())
	userID := eventID
	if result.Candidate != nil && result.Candidate.ID != "" {
		userID = result.Candidate.ID
	}

	if err := a.adkService.SessionUpsert(a.ctx, sessionID, userID, state); err != nil {
		return fmt.Sprintf("error: create session: %v", err)
	}

	questions, err := a.adkService.QuestionBankGeneratorRun(adkutils.AgentRunRequest{
		Ctx:       a.ctx,
		SessionID: sessionID,
		UserID:    userID,
		Prompt:    input,
	})
	if err != nil {
		return fmt.Sprintf("error: run agent: %v", err)
	}

	if err := a.redisCache.SaveQuestionBank(a.ctx, eventID, questions); err != nil {
		return fmt.Sprintf("error: save to redis: %v", err)
	}

	log.Printf("[qbgen] saved %d questions to redis for event=%s", len(questions), eventID)
	return "ok"
}

// GetQuestionBank returns the question bank stored in Redis for the given eventID,
// as an ordered slice (sorted by ID). Returns nil if no bank exists yet.
func (a *App) GetQuestionBank(eventID string) ([]adkutils.QuestionBankQuestion, error) {
	bank, err := a.redisCache.FindQuestionBank(a.ctx, eventID)
	if err != nil {
		return nil, err
	}
	questions := make([]adkutils.QuestionBankQuestion, 0, len(bank))
	for _, q := range bank {
		questions = append(questions, q)
	}
	return questions, nil
}

// ATSInterviewFind returns full detail for a single interview by name.
func (a *App) ATSInterviewFind(name string) (*atsclient.InterviewFindResult, error) {
	return a.atsClient.InterviewFind(name)
}

// ATSBeginSession fetches the interview from the ATS, maps its question bank to the
// ADK format, and initialises the session — all in one call. Must be called before
// StartRecording. Returns "ok" on success or a descriptive error string.
func (a *App) ATSBeginSession(interviewName string) string {
	log.Printf("[session] fetching interview %q from ATS", interviewName)

	interview, err := a.atsClient.InterviewFind(interviewName)
	if err != nil {
		msg := fmt.Sprintf("ATSBeginSession: fetch interview %q: %v", interviewName, err)
		log.Println("[session] ERROR:", msg)
		return msg
	}

	questions := atsQuestionsToADK(interview.QuestionBank.Questions)
	log.Printf("[session] interview=%q candidate=%q round=%q questions=%d",
		interviewName,
		interview.Candidate.Name,
		interview.Round.Name,
		len(questions),
	)

	// Cache first question text so we can push it to the UI the moment recording starts
	if len(questions) > 0 {
		a.initialQuestionText = questions[0].Question
	} else {
		a.initialQuestionText = ""
	}

	userID := interview.Candidate.Email
	if userID == "" {
		userID = interviewName
	}

	result := a.StartSession(userID, questions)
	if result != "ok" {
		log.Printf("[session] ERROR: StartSession failed: %s", result)
		return result
	}

	// Build interview context for judging agent
	interviewContext := fmt.Sprintf(`# Interview Context

## Candidate
- Name: %s
- Email: %s
- Designation: %s
- Skills: %s

## Job Position
- Title: %s
- Department: %s
- Location: %s

## Interview Round
- Type: %s
- Round: %s
- Expected Average Rating: %.2f

## Expected Skills
%s
`,
		interview.Candidate.Name,
		interview.Candidate.Email,
		safeString(interview.Candidate.Designation),
		strings.Join(interview.Candidate.Skills, ", "),
		interview.Job.Title,
		interview.Job.Department,
		interview.Job.Location,
		interview.Round.Type,
		interview.Round.Name,
		interview.Round.ExpectedAverageRating,
		formatExpectedSkills(interview.Round.ExpectedSkills),
	)

	// Update judging agent session with interview context
	if err := a.adkService.SetJudgingAgentContext(a.ctx, a.sessions.JudgingAgentSessionID, userID, interviewContext); err != nil {
		log.Printf("[session] WARNING: SetJudgingAgentContext: %v", err)
	} else {
		log.Printf("[session] judging agent context set — interview=%s", a.interviewID)
	}

	log.Printf("[session] ready — interview=%s user=%s questions=%d",
		a.interviewID, a.userID, len(questions))
	return "ok"
}

// safeString returns the string value if not nil, otherwise empty string
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// formatExpectedSkills formats expected skills as a bulleted list
func formatExpectedSkills(skills []atsclient.ExpectedSkill) string {
	if len(skills) == 0 {
		return "None specified"
	}
	var builder strings.Builder
	for _, skill := range skills {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", skill.Skill, skill.Description))
	}
	return strings.TrimSpace(builder.String())
}

// atsQuestionsToADK converts ATS question structs to the adkutils format expected
// by the session and orchestration pipeline.
func atsQuestionsToADK(qs []atsclient.Question) []adkutils.QuestionBankQuestion {
	out := make([]adkutils.QuestionBankQuestion, 0, len(qs))
	for _, q := range qs {
		criteria := make([]adkutils.EvaluationCriteria, 0, len(q.EvaluationCriteria))
		for _, c := range q.EvaluationCriteria {
			criteria = append(criteria, adkutils.EvaluationCriteria{
				MustMention: strings.Join(c.MustMention, ", "),
				BonusPoints: strings.Join(c.BonusPoints, ", "),
			})
		}
		triggers := make([]adkutils.FollowupTrigger, 0, len(q.FollowupTriggers))
		for _, t := range q.FollowupTriggers {
			triggers = append(triggers, adkutils.FollowupTrigger{
				Condition: t.Condition,
				FollowUp:  t.FollowUp,
			})
		}
		out = append(out, adkutils.QuestionBankQuestion{
			ID:                   q.ID,
			Category:             q.Category,
			Difficulty:           q.Difficulty,
			EstimatedTimeMinutes: q.EstimatedTimeMinutes,
			Question:             q.Question,
			IdealAnswerKeywords:  strings.Join(q.IdealAnswerKeywords, ", "),
			EvaluationCriteria:   criteria,
			FollowupTriggers:     triggers,
			PassThreshold:        q.PassThreshold,
		})
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ManualEvaluateAnswer allows the recruiter to manually trigger evaluation of the
// current answer, running both the judging agent and NQI agent. This is useful
// when the signaling agent fails to detect answer completion automatically.
func (a *App) ManualEvaluateAnswer() string {
	if a.sessions == nil {
		return "error: no active session"
	}
	if a.interviewID == "" {
		return "error: no interview ID"
	}

	log.Printf("[manual-eval] recruiter triggered manual answer evaluation — interview=%s", a.interviewID)

	// Get current question pointer
	currentQuestionID, err := a.redisCache.FindCurrentQuestionPointer(a.ctx, a.interviewID)
	if err != nil || currentQuestionID == "" {
		msg := "error: no active question found"
		log.Printf("[manual-eval] %s: %v", msg, err)
		return msg
	}

	// Get the current question and answer
	currentQuestion, err := a.redisCache.FindQuestionByID(a.ctx, a.interviewID, currentQuestionID)
	if err != nil {
		msg := fmt.Sprintf("error: question %s not found: %v", currentQuestionID, err)
		log.Printf("[manual-eval] %s", msg)
		return msg
	}

	// Get the summary to find the answer
	summary, err := a.redisCache.FindInterviewSummary(a.ctx, a.interviewID)
	if err != nil {
		msg := fmt.Sprintf("error: failed to load interview summary: %v", err)
		log.Printf("[manual-eval] %s", msg)
		return msg
	}

	// Find the answer for the current question
	var answerText string
	for _, qa := range summary.Questions {
		if qa.Question.ID == currentQuestionID {
			answerText = qa.Answer
			break
		}
	}

	if answerText == "" {
		msg := "error: no answer found for current question"
		log.Printf("[manual-eval] %s — questionID=%s", msg, currentQuestionID)
		return msg
	}

	log.Printf("[manual-eval] triggering evaluation — questionID=%s answerLen=%d", currentQuestionID, len(answerText))

	// Publish a signal_mapped event with the answer (add semicolon to mark completion)
	// This triggers the normal orchestration flow (judging + NQI)
	if err := a.redisPublisher.PublishSignalMapped(a.ctx, redispkg.SignalMappedEvent{
		InterviewID:        a.interviewID,
		UserID:             a.userID,
		Signal:             fmt.Sprintf("A:%s;", answerText), // Add semicolon to indicate completion
		QuestionID:         currentQuestionID,
		QAndA:              fmt.Sprintf("Q: %s\nA: %s", currentQuestion.Question, answerText),
		SignalingSessionID: a.sessions.SignalingAgentSessionID,
		MapperSessionID:    a.sessions.SignalingAgentMapperSessionID,
		IndicatorSessionID: a.sessions.NextQuestionIndicatorSessionID,
		ExtenderSessionID:  a.sessions.NextQuestionExtenderSessionID,
		JudgingSessionID:   a.sessions.JudgingAgentSessionID,
	}); err != nil {
		msg := fmt.Sprintf("error: failed to publish signal_mapped: %v", err)
		log.Printf("[manual-eval] %s", msg)
		return msg
	}

	log.Printf("[manual-eval] signal_mapped published — orchestration pipeline triggered")
	return "ok"
}

// AppLoginResponse combines the ATS login response and the Workable member info
type AppLoginResponse struct {
	ATSLogin *atsclient.LoginResponse `json:"ats_login"`
	Member   *workableclient.Member   `json:"member"`
}
