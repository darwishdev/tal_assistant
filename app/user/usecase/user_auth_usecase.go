package usecase

import (
	"fmt"
	"tal_assistant/pkg/atsclient"
	"tal_assistant/pkg/workableclient"
)

type UserLoginResponse struct {
	ATSLogin *atsclient.LoginResponse `json:"ats_login"`
	Member   *workableclient.Member   `json:"member"`
}

func (u *UserUseCase) UserLogin(username string, password string) (*UserLoginResponse, error) {
	atsResp, err := u.atsClient.Login(username, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	result := &UserLoginResponse{
		ATSLogin: atsResp,
	}
	opts := workableclient.ListMembersOptions{
		Email: username,
		Limit: 1,
	}
	members, err := u.workableClient.ListMembers(opts)
	if err != nil || len(members) == 0 {
		return nil, fmt.Errorf("failed to fetch workable member: %w", err)
	}
	result.Member = &members[0]
	return result, nil
}
