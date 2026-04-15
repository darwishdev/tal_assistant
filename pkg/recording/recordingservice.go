package recording

import (
	"context"
	"io"

	"tal_assistant/pkg/go_recording"
)

type ScreenSource struct {
	ID         string `json:"ID"`
	Name       string `json:"Name"`
	OffsetX    int    `json:"OffsetX"`
	OffsetY    int    `json:"OffsetY"`
	Width      int    `json:"Width"`
	Height     int    `json:"Height"`
	Screenshot string `json:"Screenshot"`
}

type AudioSource struct {
	ID         string // Device ID
	Name       string // Human-readable name
	Type       string // "mic", "speaker"
	IsDefault  bool   // Whether it is the default device
	IsPersonal bool   // True for headphones/headsets, false for room speakers
}

type RecordingService struct {
	svc *go_recording.RecordingService
}

func NewRecordingService() *RecordingService {
	return &RecordingService{
		svc: go_recording.NewRecordingService(),
	}
}

func (r *RecordingService) Start(micID, speakerID string) (io.ReadCloser, int, error) {
	return r.svc.Start(micID, speakerID)
}

func (r *RecordingService) StartScreenRecording(
	screen *ScreenSource,
	micID, speakerID string,
	outputDir string,
) (string, error) {
	var gScreen *go_recording.ScreenSource
	if screen != nil {
		gScreen = &go_recording.ScreenSource{
			ID:         screen.ID,
			Name:       screen.Name,
			OffsetX:    screen.OffsetX,
			OffsetY:    screen.OffsetY,
			Width:      screen.Width,
			Height:     screen.Height,
			Screenshot: screen.Screenshot,
		}
	}
	return r.svc.StartScreenRecording(gScreen, micID, speakerID, outputDir)
}

func (r *RecordingService) StopScreenRecording() {
	r.svc.StopScreenRecording()
}

func (r *RecordingService) Stop() {
	r.svc.Stop()
}

func (r *RecordingService) AudioDeviceList(ctx context.Context) ([]AudioSource, error) {
	devices, err := go_recording.ListDevices()
	if err != nil {
		return nil, err
	}

	var sources []AudioSource
	for _, d := range devices {
		sources = append(sources, AudioSource{
			ID:         d.ID,
			Name:       d.Name,
			Type:       string(d.Flow),
			IsDefault:  d.IsDefault,
			IsPersonal: d.IsPersonal,
		})
	}
	return sources, nil
}

func (r *RecordingService) ScreenDeviceList(ctx context.Context) ([]ScreenSource, error) {
	screens, err := r.svc.ScreenDeviceList(ctx)
	if err != nil {
		return nil, err
	}

	var sources []ScreenSource
	for _, s := range screens {
		sources = append(sources, ScreenSource{
			ID:         s.ID,
			Name:       s.Name,
			OffsetX:    s.OffsetX,
			OffsetY:    s.OffsetY,
			Width:      s.Width,
			Height:     s.Height,
			Screenshot: s.Screenshot,
		})
	}
	return sources, nil
}
