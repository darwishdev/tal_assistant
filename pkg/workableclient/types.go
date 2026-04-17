package workableclient

// ---------------------------------------------------------------------------
// Shared / Primitive
// ---------------------------------------------------------------------------

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

type DepartmentNode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

// JobState mirrors WorkableJobState.
type JobState string

const (
	JobStateDraft     JobState = "draft"
	JobStatePublished JobState = "published"
	JobStateArchived  JobState = "archived"
	JobStateClosed    JobState = "closed"
)

type Job struct {
	ID                  string           `json:"id"`
	Title               string           `json:"title"`
	FullTitle           string           `json:"full_title"`
	Shortcode           string           `json:"shortcode"`
	Code                *string          `json:"code,omitempty"`
	State               JobState         `json:"state"`
	Sample              bool             `json:"sample"`
	Confidential        bool             `json:"confidential"`
	Department          *string          `json:"department,omitempty"`
	DepartmentHierarchy []DepartmentNode `json:"department_hierarchy,omitempty"`
	URL                 string           `json:"url"`
	ApplicationURL      string           `json:"application_url"`
	Shortlink           string           `json:"shortlink"`
	WorkplaceType       string           `json:"workplace_type"`
	Location            Location         `json:"location"`
	Locations           []Location       `json:"locations,omitempty"`
	Salary              Salary           `json:"salary"`
	CreatedAt           string           `json:"created_at"`
	UpdatedAt           string           `json:"updated_at"`
	Keywords            []string         `json:"keywords,omitempty"`
	// Detail-only (GET /jobs/{shortcode})
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

type ListJobsOptions struct {
	State              JobState
	Limit              int
	SinceID            string
	UpdatedAfter       string
	IncludeDescription bool
	Paginate           bool
}

// ---------------------------------------------------------------------------
// Stages
// ---------------------------------------------------------------------------

type Stage struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Position int    `json:"position"`
}

// ---------------------------------------------------------------------------
// Candidates
// ---------------------------------------------------------------------------

type ResumeMetadata struct {
	Filename  *string `json:"filename,omitempty"`
	Filetype  *string `json:"filetype,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type SocialProfile struct {
	Type string `json:"type"`
	Name string `json:"name"`
	URL  string `json:"url"`
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
	ID       string  `json:"id"`
	Title    *string `json:"title,omitempty"`
	Summary  *string `json:"summary,omitempty"`
	StartDate *string `json:"start_date,omitempty"`
	EndDate  *string `json:"end_date,omitempty"`
	Company  *string `json:"company,omitempty"`
	Industry *string `json:"industry,omitempty"`
	Current  bool    `json:"current"`
}

type Candidate struct {
	ID                     string           `json:"id"`
	Name                   string           `json:"name"`
	Firstname              string           `json:"firstname"`
	Lastname               string           `json:"lastname"`
	Headline               *string          `json:"headline,omitempty"`
	Account                map[string]string `json:"account,omitempty"`
	Job                    map[string]string `json:"job,omitempty"`
	Stage                  string           `json:"stage"`
	StageKind              string           `json:"stage_kind"`
	Disqualified           bool             `json:"disqualified"`
	DisqualificationReason *string          `json:"disqualification_reason,omitempty"`
	HiredAt                *string          `json:"hired_at,omitempty"`
	MovedToOfferAt         *string          `json:"moved_to_offer_at,omitempty"`
	Sourced                bool             `json:"sourced"`
	ProfileURL             string           `json:"profile_url"`
	Address                *string          `json:"address,omitempty"`
	Phone                  *string          `json:"phone,omitempty"`
	Email                  *string          `json:"email,omitempty"`
	Domain                 *string          `json:"domain,omitempty"`
	Outlet                 *string          `json:"outlet,omitempty"`
	CommonSource           *string          `json:"common_source,omitempty"`
	CommonSourceCategory   *string          `json:"common_source_category,omitempty"`
	CreatedAt              string           `json:"created_at"`
	UpdatedAt              string           `json:"updated_at"`
	ResumeMetadata         *ResumeMetadata  `json:"resume_metadata,omitempty"`
	// Detail-only
	ImageURL         *string           `json:"image_url,omitempty"`
	CoverLetter      *string           `json:"cover_letter,omitempty"`
	Summary          *string           `json:"summary,omitempty"`
	EducationEntries []EducationEntry  `json:"education_entries,omitempty"`
	ExperienceEntries []ExperienceEntry `json:"experience_entries,omitempty"`
	Skills           []any             `json:"skills,omitempty"`
	Answers          []any             `json:"answers,omitempty"`
	ResumeURL        *string           `json:"resume_url,omitempty"`
	SocialProfiles   []SocialProfile   `json:"social_profiles,omitempty"`
	DisqualifiedAt   *string           `json:"disqualified_at,omitempty"`
	Withdrew         bool              `json:"withdrew,omitempty"`
	Location         *Location         `json:"location,omitempty"`
}

type ListCandidatesOptions struct {
	Stage        string
	Limit        int
	SinceID      string
	UpdatedAfter string
	Paginate     bool
}

// ---------------------------------------------------------------------------
// Activities
// ---------------------------------------------------------------------------

type Activity struct {
	ID          string         `json:"id"`
	Action      string         `json:"action"`
	StageName   *string        `json:"stage_name,omitempty"`
	ActionStage map[string]any `json:"action_stage,omitempty"`
	TargetStage map[string]any `json:"target_stage,omitempty"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	Member      map[string]string `json:"member,omitempty"`
	Body        *string        `json:"body,omitempty"`
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

type EventMember struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type Conference struct {
	Type *string `json:"type,omitempty"`
	URL  *string `json:"url,omitempty"`
}

type Event struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description *string      `json:"description,omitempty"`
	Type        string       `json:"type"`
	StartsAt    string       `json:"starts_at"`
	EndsAt      string       `json:"ends_at"`
	Cancelled   bool         `json:"cancelled"`
	Job         map[string]string `json:"job"`
	Candidate   map[string]string `json:"candidate"`
	Members     []EventMember `json:"members,omitempty"`
	Conference  *Conference  `json:"conference,omitempty"`
}

type ListEventsOptions struct {
	EventType         string
	Limit             int
	SinceID           string
	IncludeCancelled  bool
	StartDate         string
	EndDate           string
	Paginate          bool
	MemberID          string
}

// ---------------------------------------------------------------------------
// Subscriptions (webhooks)
// ---------------------------------------------------------------------------

// EventType mirrors WorkableEventType.
type EventType string

const (
	EventTypeCandidateCreated EventType = "candidate_created"
	EventTypeCandidateUpdated EventType = "candidate_updated"
	EventTypeCandidateMoved   EventType = "candidate_moved"
	EventTypeCandidateHired   EventType = "candidate_hired"
	EventTypeJobPublished     EventType = "job_published"
	EventTypeJobClosed        EventType = "job_closed"
	EventTypeJobArchived      EventType = "job_archived"
)

type Subscription struct {
	ID           int     `json:"id"`
	Event        string  `json:"event"`
	Target       string  `json:"target"`
	ValidUntil   *string `json:"valid_until,omitempty"`
	CreatedAt    string  `json:"created_at"`
	StageSlug    *string `json:"stage_slug,omitempty"`
	JobShortcode *string `json:"job_shortcode,omitempty"`
}

type CreateSubscriptionRequest struct {
	Event        EventType
	TargetURL    string
	StageSlug    string // optional
	JobShortcode string // optional
}

// EventFindResult combines a single event with the full job and candidate records
// referenced by that event.
type EventFindResult struct {
	Event     *Event     `json:"event"`
	Job       *Job       `json:"job"`
	Candidate *Candidate `json:"candidate"`
}

// ---------------------------------------------------------------------------
// Internal envelope types (unexported)
// ---------------------------------------------------------------------------

type jobsEnvelope struct {
	Jobs   []Job              `json:"jobs"`
	Paging map[string]string  `json:"paging"`
}

type stagesEnvelope struct {
	Stages []Stage `json:"stages"`
}

type candidatesEnvelope struct {
	Candidates []Candidate        `json:"candidates"`
	Paging     map[string]string  `json:"paging"`
}

type candidateEnvelope struct {
	Candidate Candidate `json:"candidate"`
}

type activitiesEnvelope struct {
	Activities []Activity `json:"activities"`
}

type eventsEnvelope struct {
	Events []Event            `json:"events"`
	Paging map[string]string  `json:"paging"`
}

type subscriptionsEnvelope struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

type subscriptionEnvelope struct {
	Subscription Subscription `json:"subscription"`
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

type CommentDetail struct {
	Body string `json:"body"`
}

type CommentCreateRequest struct {
	MemberID string        `json:"member_id,omitempty"`
	Comment  CommentDetail `json:"comment"`
}

type Comment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type commentEnvelope struct {
	Comment Comment `json:"comment"`
}

// ---------------------------------------------------------------------------
// Members
// ---------------------------------------------------------------------------

type MemberRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Member struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Role     string   `json:"role"`
	Headline *string  `json:"headline,omitempty"`
	Type     string   `json:"type,omitempty"`
	HRISRole string   `json:"hris_role,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	Active   bool     `json:"active"`
}

type ListMembersOptions struct {
	Limit    int
	SinceID  string
	Email    string
	Status   string
	Paginate bool
}

type membersEnvelope struct {
	Members []Member          `json:"members"`
	Paging  map[string]string `json:"paging"`
}
