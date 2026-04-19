package adapter

import (
	"tal_assistant/app/event/dto"
	"tal_assistant/pkg/workableclient"
)

func (a *EventAdapter) eventFromWorkable(in *workableclient.Event) *dto.Event {
	if in == nil {
		return nil
	}

	return &dto.Event{
		ID:          in.ID,
		Title:       in.Title,
		Description: in.Description,
		Type:        in.Type,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		Cancelled:   in.Cancelled,
		Job:         in.Job,
		Candidate:   in.Candidate,
		Members:     a.eventMembersFromWorkable(in.Members),
		Conference:  a.conferenceFromWorkable(in.Conference),
	}
}
func (a *EventAdapter) jobFromWorkable(in *workableclient.Job) *dto.Job {
	if in == nil {
		return nil
	}

	return &dto.Job{
		ID:              in.ID,
		Title:           in.Title,
		FullTitle:       in.FullTitle,
		Shortcode:       in.Shortcode,
		Code:            in.Code,
		State:           dto.JobState(in.State),
		Sample:          in.Sample,
		Confidential:    in.Confidential,
		Department:      in.Department,
		URL:             in.URL,
		ApplicationURL:  in.ApplicationURL,
		Shortlink:       in.Shortlink,
		WorkplaceType:   in.WorkplaceType,
		Location:        a.locationFromWorkable(in.Location),
		Salary:          a.salaryFromWorkable(in.Salary),
		CreatedAt:       in.CreatedAt,
		UpdatedAt:       in.UpdatedAt,
		Keywords:        in.Keywords,
		FullDescription: in.FullDescription,
		Description:     in.Description,
		Requirements:    in.Requirements,
		Benefits:        in.Benefits,
		EmploymentType:  in.EmploymentType,
		Industry:        in.Industry,
		Function:        in.Function,
		Experience:      in.Experience,
		Education:       in.Education,
	}
}
func (a *EventAdapter) candidateFromWorkable(in *workableclient.Candidate) *dto.Candidate {
	if in == nil {
		return nil
	}
	location := dto.Location{}
	if in.Location != nil {
		location = a.locationFromWorkable(*in.Location)
	}
	return &dto.Candidate{
		ID:                   in.ID,
		Name:                 in.Name,
		Firstname:            in.Firstname,
		Lastname:             in.Lastname,
		Headline:             in.Headline,
		Account:              in.Account,
		Job:                  in.Job,
		Stage:                in.Stage,
		StageKind:            in.StageKind,
		Sourced:              in.Sourced,
		ProfileURL:           in.ProfileURL,
		Address:              in.Address,
		Phone:                in.Phone,
		Email:                in.Email,
		Domain:               in.Domain,
		Outlet:               in.Outlet,
		CommonSource:         in.CommonSource,
		CommonSourceCategory: in.CommonSourceCategory,
		CreatedAt:            in.CreatedAt,
		UpdatedAt:            in.UpdatedAt,
		ImageURL:             in.ImageURL,
		CoverLetter:          in.CoverLetter,
		Summary:              in.Summary,
		EducationEntries:     a.educationFromWorkable(in.EducationEntries),
		ExperienceEntries:    a.experienceFromWorkable(in.ExperienceEntries),
		Skills:               in.Skills,
		Answers:              in.Answers,
		ResumeURL:            in.ResumeURL,
		SocialProfiles:       a.socialProfilesFromWorkable(in.SocialProfiles),
		DisqualifiedAt:       in.DisqualifiedAt,
		Withdrew:             in.Withdrew,
		Location:             location,
	}
}
func (a *EventAdapter) eventMembersFromWorkable(in []workableclient.EventMember) []dto.EventMember {
	if len(in) == 0 {
		return nil
	}
	out := make([]dto.EventMember, len(in))
	for i, m := range in {
		out[i] = dto.EventMember{
			ID:     m.ID,
			Name:   m.Name,
			Type:   m.Type,
			Status: m.Status,
		}
	}
	return out
}

func (a *EventAdapter) conferenceFromWorkable(in *workableclient.Conference) *dto.EventConference {
	if in == nil {
		return nil
	}
	return &dto.EventConference{
		Type: in.Type,
		URL:  in.URL,
	}
}

func (a *EventAdapter) locationFromWorkable(in workableclient.Location) dto.Location {
	return dto.Location{
		LocationStr:   in.LocationStr,
		Country:       in.Country,
		CountryCode:   in.CountryCode,
		Region:        in.Region,
		RegionCode:    in.RegionCode,
		City:          in.City,
		ZipCode:       in.ZipCode,
		Telecommuting: in.Telecommuting,
		WorkplaceType: in.WorkplaceType,
	}
}

func (a *EventAdapter) salaryFromWorkable(in workableclient.Salary) dto.Salary {
	return dto.Salary{
		Currency: in.Currency,
		MinValue: in.MinValue,
		MaxValue: in.MaxValue,
		Per:      in.Per,
	}
}

func (a *EventAdapter) educationFromWorkable(entries []workableclient.EducationEntry) []dto.EducationEntry {
	if entries == nil {
		return nil
	}

	result := make([]dto.EducationEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, dto.EducationEntry{
			ID:           e.ID,
			Degree:       e.Degree,
			School:       e.School,
			FieldOfStudy: e.FieldOfStudy,
			StartDate:    e.StartDate,
			EndDate:      e.EndDate,
		})
	}

	return result
}

func (a *EventAdapter) experienceFromWorkable(entries []workableclient.ExperienceEntry) []dto.ExperienceEntry {
	if entries == nil {
		return nil
	}

	result := make([]dto.ExperienceEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, dto.ExperienceEntry{
			ID:        e.ID,
			Title:     e.Title,
			Summary:   e.Summary,
			StartDate: e.StartDate,
			EndDate:   e.EndDate,
			Company:   e.Company,
			Industry:  e.Industry,
			Current:   e.Current,
		})
	}

	return result
}

func (a *EventAdapter) socialProfilesFromWorkable(profiles []workableclient.SocialProfile) []dto.SocialProfile {
	if profiles == nil {
		return nil
	}

	result := make([]dto.SocialProfile, 0, len(profiles))
	for _, p := range profiles {
		result = append(result, dto.SocialProfile{
			Type: p.Type,
			Name: p.Name,
			URL:  p.URL,
		})
	}

	return result
}
