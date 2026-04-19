package usecase

import (
	"tal_assistant/pkg/atsclient"
	"tal_assistant/pkg/workableclient"
)

type ListSourcesResonse struct {
	AudioDevices []string `json:"audio_devices"`
}
type SessionUseCaseInterface interface {
	SessionStart(eventID string) string
	SessionFind(sessionID string) string
	SessionStop(sessionID string) string
	DeviceList() (*ListSourcesResonse, error)
	CheckGoogleDriveAuthorization() (*atsclient.DriveAuthStatus, error)
}
type SessionUseCase struct {
	atsClient      atsclient.ATSClientInterface
	workableClient workableclient.ClientInterface
}

func NewSessionUseCase(
	atsClient atsclient.ATSClientInterface,
	workableClient workableclient.ClientInterface,
) *SessionUseCase {
	return &SessionUseCase{
		atsClient:      atsClient,
		workableClient: workableClient,
	}
}
