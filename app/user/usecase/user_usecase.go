package usecase

import (
	"tal_assistant/pkg/atsclient"
	"tal_assistant/pkg/workableclient"
)

type UserUseCaseInterface interface {
	UserLogin(username string, password string) (UserLoginResponse, error)
}
type UserUseCase struct {
	atsClient      atsclient.ATSClientInterface
	workableClient workableclient.ClientInterface
}

func NewUserUseCase(
	atsClient atsclient.ATSClientInterface,
	workableClient workableclient.ClientInterface,
) *UserUseCase {
	return &UserUseCase{
		atsClient:      atsClient,
		workableClient: workableClient,
	}
}
