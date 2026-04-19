package dto

import "tal_assistant/pkg/adkutils"

type EventListRequest struct {
	EventType        string
	Limit            int
	SinceID          string
	IncludeCancelled bool
	StartDate        string
	EndDate          string
	Paginate         bool
	MemberID         string
}

type EventListRow struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Type        string  `json:"type"`
	StartsAt    string  `json:"starts_at"`
	EndsAt      string  `json:"ends_at"`
	Cancelled   bool    `json:"cancelled"`
}
type EventListResponse struct {
	Events []EventListRow `json:"events"`
}

type EventMember struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type EventConference struct {
	Type *string `json:"type,omitempty"`
	URL  *string `json:"url,omitempty"`
}
type Event struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description *string           `json:"description,omitempty"`
	Type        string            `json:"type"`
	StartsAt    string            `json:"starts_at"`
	EndsAt      string            `json:"ends_at"`
	Cancelled   bool              `json:"cancelled"`
	Job         map[string]string `json:"job"`
	Candidate   map[string]string `json:"candidate"`
	Members     []EventMember     `json:"members,omitempty"`
	Conference  *EventConference  `json:"conference,omitempty"`
}

type JobState string

const (
	JobStateDraft     JobState = "draft"
	JobStatePublished JobState = "published"
	JobStateArchived  JobState = "archived"
	JobStateClosed    JobState = "closed"
)

type Location struct {
	LocationStr   string  `json:"location_str,omitempty"`
	Country       string  `json:"country,omitempty"`
	CountryCode   string  `json:"country_code,omitempty"`
	Region        *string `json:"region,omitempty"`
	RegionCode    *string `json:"region_code,omitempty"`
	City          *string `json:"city,omitempty"`
	ZipCode       *string `json:"zip_code,omitempty"`
	Telecommuting bool    `json:"telecommuting,omitempty"`
	WorkplaceType string  `json:"workplace_type,omitempty"`
}

type Salary struct {
	Currency *string  `json:"salary_currency,omitempty"`
	MinValue *float64 `json:"min_value,omitempty"`
	MaxValue *float64 `json:"max_value,omitempty"`
	Per      *string  `json:"salary_per,omitempty"`
}

type Job struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	FullTitle      string   `json:"full_title"`
	Shortcode      string   `json:"shortcode"`
	Code           *string  `json:"code,omitempty"`
	State          JobState `json:"state"`
	Sample         bool     `json:"sample"`
	Confidential   bool     `json:"confidential"`
	Department     *string  `json:"department,omitempty"`
	URL            string   `json:"url"`
	ApplicationURL string   `json:"application_url"`
	Shortlink      string   `json:"shortlink"`
	WorkplaceType  string   `json:"workplace_type"`
	Location       Location `json:"location"`
	Salary         Salary   `json:"salary"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	Keywords       []string `json:"keywords,omitempty"`

	FullDescription *string `json:"full_description,omitempty"`
	Description     *string `json:"description,omitempty"`
	Requirements    *string `json:"requirements,omitempty"`
	Benefits        *string `json:"benefits,omitempty"`
	EmploymentType  *string `json:"employment_type,omitempty"`
	Industry        *string `json:"industry,omitempty"`
	Function        *string `json:"function,omitempty"`
	Experience      *string `json:"experience,omitempty"`
	Education       *string `json:"education,omitempty"`
}

type EducationEntry struct {
	ID           string  `json:"id"`
	Degree       *string `json:"degree,omitempty"`
	School       *string `json:"school,omitempty"`
	FieldOfStudy *string `json:"field_of_study,omitempty"`
	StartDate    *string `json:"start_date,omitempty"`
	EndDate      *string `json:"end_date,omitempty"`
}

type ExperienceEntry struct {
	ID        string  `json:"id"`
	Title     *string `json:"title,omitempty"`
	Summary   *string `json:"summary,omitempty"`
	StartDate *string `json:"start_date,omitempty"`
	EndDate   *string `json:"end_date,omitempty"`
	Company   *string `json:"company,omitempty"`
	Industry  *string `json:"industry,omitempty"`
	Current   bool    `json:"current"`
}

type SocialProfile struct {
	Type string `json:"type"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Candidate struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Firstname            string            `json:"firstname"`
	Lastname             string            `json:"lastname"`
	Headline             *string           `json:"headline,omitempty"`
	Account              map[string]string `json:"account,omitempty"`
	Job                  map[string]string `json:"job,omitempty"`
	Stage                string            `json:"stage"`
	StageKind            string            `json:"stage_kind"`
	Sourced              bool              `json:"sourced"`
	ProfileURL           string            `json:"profile_url"`
	Address              *string           `json:"address,omitempty"`
	Phone                *string           `json:"phone,omitempty"`
	Email                *string           `json:"email,omitempty"`
	Domain               *string           `json:"domain,omitempty"`
	Outlet               *string           `json:"outlet,omitempty"`
	CommonSource         *string           `json:"common_source,omitempty"`
	CommonSourceCategory *string           `json:"common_source_category,omitempty"`
	CreatedAt            string            `json:"created_at"`
	UpdatedAt            string            `json:"updated_at"`

	ImageURL          *string           `json:"image_url,omitempty"`
	CoverLetter       *string           `json:"cover_letter,omitempty"`
	Summary           *string           `json:"summary,omitempty"`
	EducationEntries  []EducationEntry  `json:"education_entries,omitempty"`
	ExperienceEntries []ExperienceEntry `json:"experience_entries,omitempty"`
	Skills            []any             `json:"skills,omitempty"`
	Answers           []any             `json:"answers,omitempty"`
	ResumeURL         *string           `json:"resume_url,omitempty"`
	SocialProfiles    []SocialProfile   `json:"social_profiles,omitempty"`
	DisqualifiedAt    *string           `json:"disqualified_at,omitempty"`
	Withdrew          bool              `json:"withdrew,omitempty"`
	Location          Location          `json:"location,omitempty"`
}

type EventFindResponse struct {
	Event        *Event                          `json:"event"`
	Job          *Job                            `json:"job"`
	Candidate    *Candidate                      `json:"candidate"`
	QuestionBank []adkutils.QuestionBankQuestion `json:"question_bank,omitempty"`
}
