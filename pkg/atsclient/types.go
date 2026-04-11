package atsclient

// ── Login ──────────────────────────────────────────────────────────────────

// LoginResponse matches: {"message":"Logged In","home_page":"desk","full_name":"Administrator"}
type LoginResponse struct {
	Message  string `json:"message"`
	HomePage string `json:"home_page"`
	FullName string `json:"full_name"`
}

// ── Interview List ─────────────────────────────────────────────────────────

// InterviewListItem matches one entry in the interview_list message array.
type InterviewListItem struct {
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	ScheduledOn     string  `json:"scheduled_on"`
	FromTime        string  `json:"from_time"`
	ToTime          string  `json:"to_time"`
	InterviewRound  string  `json:"interview_round"`
	JobApplicant    string  `json:"job_applicant"`
	CandidateName   string  `json:"candidate_name"`
	CandidateEmail  string  `json:"candidate_email"`
	JobOpening      *string `json:"job_opening"`
	JobTitle        *string `json:"job_title"`
}

// interviewListResponse is the raw envelope from the API.
type interviewListResponse struct {
	Message []InterviewListItem `json:"message"`
}

// ── Interview Find ─────────────────────────────────────────────────────────

type InterviewDetail struct {
	ID                    string  `json:"id"`
	ScheduledOn           string  `json:"scheduled_on"`
	Designation           *string `json:"designation"`
	ExpectedAverageRating float64 `json:"expected_average_rating"`
	Status                string  `json:"status"`
}

type CandidateExperience struct {
	Company          string   `json:"company"`
	Role             string   `json:"role"`
	From             string   `json:"from"`
	To               string   `json:"to"`
	Responsibilities []string `json:"responsibilities"`
}

type CandidateEducation struct {
	Degree      string `json:"degree"`
	Institution string `json:"institution"`
	Year        string `json:"year"`
}

type CandidateProject struct {
	Name        string   `json:"name"`
	Description []string `json:"description"`
}

type Candidate struct {
	Name        string                `json:"name"`
	Email       string                `json:"email"`
	Phone       string                `json:"phone"`
	Designation *string               `json:"designation"`
	Summary     string                `json:"summary"`
	Skills      []string              `json:"skills"`
	Experience  []CandidateExperience `json:"experience"`
	Education   []CandidateEducation  `json:"education"`
	Projects    []CandidateProject    `json:"projects"`
}

type JobDescriptionSection struct {
	Title       string   `json:"title"`
	Description *string  `json:"description"`
	Points      []string `json:"points"`
}

type PipelineStep struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Job struct {
	ID                  string                  `json:"id"`
	Title               string                  `json:"title"`
	Designation         string                  `json:"designation"`
	Department          string                  `json:"department"`
	Location            string                  `json:"location"`
	Description         []JobDescriptionSection `json:"description"`
	CurrentPipelineStep PipelineStep            `json:"current_pipeline_step"`
}

type ExpectedSkill struct {
	Skill       string `json:"skill"`
	Description string `json:"description"`
}

type InterviewRound struct {
	Name                  string          `json:"name"`
	Type                  string          `json:"type"`
	Designation           string          `json:"designation"`
	ExpectedAverageRating float64         `json:"expected_average_rating"`
	ExpectedSkills        []ExpectedSkill `json:"expected_skills"`
}

type EvaluationCriteria struct {
	MustMention []string `json:"must_mention"`
	BonusPoints []string `json:"bonus_points"`
}

type FollowupTrigger struct {
	Condition string `json:"condition"`
	FollowUp  string `json:"follow_up"`
}

type Question struct {
	ID                   string               `json:"id"`
	Category             string               `json:"category"`
	Difficulty           string               `json:"difficulty"`
	EstimatedTimeMinutes int                  `json:"estimated_time_minutes"`
	Question             string               `json:"question"`
	IdealAnswerKeywords  []string             `json:"ideal_answer_keywords"`
	EvaluationCriteria   []EvaluationCriteria `json:"evaluation_criteria"`
	FollowupTriggers     []FollowupTrigger    `json:"followup_triggers"`
	PassThreshold        float64              `json:"pass_threshold"`
}

type QuestionBank struct {
	Name       string     `json:"name"`
	FocusAreas []string   `json:"focus_areas"`
	Questions  []Question `json:"questions"`
}

// InterviewFindResult is the fully decoded payload from interview_find.
type InterviewFindResult struct {
	Interview    InterviewDetail `json:"interview"`
	Candidate    Candidate       `json:"candidate"`
	Job          Job             `json:"job"`
	Round        InterviewRound  `json:"round"`
	QuestionBank QuestionBank    `json:"question_bank"`
}

// interviewFindResponse is the raw envelope from the API.
type interviewFindResponse struct {
	Message InterviewFindResult `json:"message"`
}
