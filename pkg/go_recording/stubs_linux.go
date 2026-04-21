//go:build linux

package go_recording

import (
	"context"
	"fmt"
	"io"
)

type Flow string
type FormFactor string

const (
	FlowSpeaker Flow = "speaker"
	FlowMic     Flow = "mic"
)

type Device struct {
	ID         string
	Name       string
	Flow       Flow
	FormFactor FormFactor
	IsDefault  bool
	IsPersonal bool
}

type ScreenSource struct {
	ID         string
	Name       string
	OffsetX    int
	OffsetY    int
	Width      int
	Height     int
	Screenshot string
}

type RecordingService struct{}

func NewRecordingService() *RecordingService {
	return &RecordingService{}
}

func (s *RecordingService) SetFFmpegLog(w io.Writer) {}

func (s *RecordingService) Start(micID, speakerID string) (io.ReadCloser, int, error) {
	return nil, 0, fmt.Errorf("recording not implemented on linux")
}

func (s *RecordingService) StartScreenRecording(
	screen *ScreenSource,
	micID, speakerID string,
	outputDir string,
) (string, error) {
	return "", fmt.Errorf("screen recording not implemented on linux")
}

func (s *RecordingService) StopScreenRecording() {}

func (s *RecordingService) Stop() {}

func (s *RecordingService) ScreenDeviceList(ctx context.Context) ([]ScreenSource, error) {
	return nil, nil
}

func ListDevices() ([]Device, error) {
	return nil, nil
}
