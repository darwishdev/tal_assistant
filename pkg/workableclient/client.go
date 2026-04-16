// Package workableclient provides an HTTP client for the Workable SPI v3 API.
// Authentication is Bearer-token-based: the token and subdomain are provided
// on construction and sent on every request via the Authorization header.
//
// Usage:
//
//	client, err := workableclient.New("mycompany", "my-api-token")
//	if err != nil { ... }
//	defer client.Close()
//
//	jobs, err := client.ListJobs(workableclient.ListJobsOptions{
//	    State:    workableclient.JobStatePublished,
//	    Paginate: true,
//	})
package workableclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultLimit   = 50
	maxPages       = 200
	defaultTimeout = 30 * time.Second
)

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

// APIError is returned when Workable responds with a non-2xx status code.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("workableclient: API error %d: %s", e.StatusCode, e.Message)
}

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

// ClientInterface defines all Workable SPI v3 operations.
type ClientInterface interface {
	// Jobs
	ListJobs(opts ListJobsOptions) ([]Job, error)
	GetJob(shortcode string) (*Job, error)

	// Stages
	ListStages() ([]Stage, error)

	// Candidates
	ListCandidates(opts ListCandidatesOptions) ([]Candidate, error)
	ListJobCandidates(shortcode string, opts ListCandidatesOptions) ([]Candidate, error)
	GetCandidate(shortcode, candidateID string) (*Candidate, error)
	GetCandidateActivities(shortcode, candidateID string) ([]Activity, error)

	// Events
	ListEvents(opts ListEventsOptions) ([]Event, error)
	ListFutureEvents(opts ListEventsOptions) ([]Event, error)

	// Members
	ListMembers(opts ListMembersOptions) ([]Member, error)

	// Subscriptions (webhooks)
	ListSubscriptions() ([]Subscription, error)
	CreateSubscription(req CreateSubscriptionRequest) (*Subscription, error)
	DeleteSubscription(id int) error

	// Close releases the underlying HTTP transport.
	Close()
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client is the concrete implementation of ClientInterface.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a ready-to-use Workable API client.
// subdomain is the company slug (e.g. "lucidya").
// token is the Workable API token (Bearer credential).
func New(subdomain, token string) (ClientInterface, error) {
	transport := &bearerTransport{
		token:   token,
		wrapped: http.DefaultTransport,
	}
	return &Client{
		baseURL: fmt.Sprintf("https://%s.workable.com/spi/v3", subdomain),
		http: &http.Client{
			Transport: transport,
			Timeout:   defaultTimeout,
		},
	}, nil
}

// Close is a no-op for the default transport; present to satisfy the interface
// and allow future resource cleanup (e.g. connection pool draining).
func (c *Client) Close() {}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

func (c *Client) ListJobs(opts ListJobsOptions) ([]Job, error) {
	params := url.Values{}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	params.Set("limit", strconv.Itoa(limit))
	if opts.State != "" {
		params.Set("state", string(opts.State))
	}
	if opts.SinceID != "" {
		params.Set("since_id", opts.SinceID)
	}
	if opts.UpdatedAfter != "" {
		params.Set("updated_after", opts.UpdatedAfter)
	}
	if opts.IncludeDescription {
		params.Set("include_fields", "description,full_description,requirements,benefits")
	}

	return paginateList[Job, *jobsEnvelope](c, "jobs", params, "jobs", opts.Paginate)
}

func (c *Client) GetJob(shortcode string) (*Job, error) {
	var env struct {
		Job Job `json:"job"`
	}
	if err := c.get("jobs/"+shortcode, nil, &env); err != nil {
		return nil, err
	}
	return &env.Job, nil
}

// ---------------------------------------------------------------------------
// Stages
// ---------------------------------------------------------------------------

func (c *Client) ListStages() ([]Stage, error) {
	var env stagesEnvelope
	if err := c.get("stages", nil, &env); err != nil {
		return nil, err
	}
	return env.Stages, nil
}

// ---------------------------------------------------------------------------
// Candidates
// ---------------------------------------------------------------------------

func (c *Client) ListCandidates(opts ListCandidatesOptions) ([]Candidate, error) {
	params := candidateParams(opts)
	return paginateList[Candidate, *candidatesEnvelope](c, "candidates", params, "candidates", opts.Paginate)
}

func (c *Client) ListJobCandidates(shortcode string, opts ListCandidatesOptions) ([]Candidate, error) {
	params := candidateParams(opts)
	path := "jobs/" + shortcode + "/candidates"
	return paginateList[Candidate, *candidatesEnvelope](c, path, params, "candidates", opts.Paginate)
}

func (c *Client) GetCandidate(shortcode, candidateID string) (*Candidate, error) {
	var env candidateEnvelope
	path := fmt.Sprintf("jobs/%s/candidates/%s", shortcode, candidateID)
	if err := c.get(path, nil, &env); err != nil {
		return nil, err
	}
	return &env.Candidate, nil
}

func (c *Client) GetCandidateActivities(shortcode, candidateID string) ([]Activity, error) {
	var env activitiesEnvelope
	path := fmt.Sprintf("jobs/%s/candidates/%s/activities", shortcode, candidateID)
	if err := c.get(path, nil, &env); err != nil {
		return nil, err
	}
	return env.Activities, nil
}

func candidateParams(opts ListCandidatesOptions) url.Values {
	params := url.Values{}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	params.Set("limit", strconv.Itoa(limit))
	if opts.Stage != "" {
		params.Set("stage", opts.Stage)
	}
	if opts.SinceID != "" {
		params.Set("since_id", opts.SinceID)
	}
	if opts.UpdatedAfter != "" {
		params.Set("updated_after", opts.UpdatedAfter)
	}
	return params
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func (c *Client) ListEvents(opts ListEventsOptions) ([]Event, error) {
	params := url.Values{}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	params.Set("limit", strconv.Itoa(limit))

	eventType := opts.EventType
	if eventType == "" {
		eventType = "interview"
	}
	params.Set("type", eventType)

	if opts.IncludeCancelled {
		params.Set("include_cancelled", "true")
	} else {
		params.Set("include_cancelled", "false")
	}
	if opts.SinceID != "" {
		params.Set("since_id", opts.SinceID)
	}
	if opts.StartDate != "" {
		params.Set("start_date", opts.StartDate)
	}
	if opts.EndDate != "" {
		params.Set("end_date", opts.EndDate)
	}
	if opts.MemberID != "" {
		params.Set("member_id", opts.MemberID)
	}

	return paginateList[Event, *eventsEnvelope](c, "events", params, "events", opts.Paginate)
}

func (c *Client) ListFutureEvents(opts ListEventsOptions) ([]Event, error) {
	now := time.Now().UTC()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	opts.StartDate = startOfToday.Format("2006-01-02T15:04:05.000Z")

	events, err := c.ListEvents(opts)
	if err != nil {
		return nil, err
	}

	var futureEvents []Event
	for _, ev := range events {
		endTime, err := time.Parse(time.RFC3339, ev.EndsAt)
		if err != nil {
			futureEvents = append(futureEvents, ev)
			continue
		}
		if endTime.After(now) {
			futureEvents = append(futureEvents, ev)
		}
	}
	return futureEvents, nil
}

// ---------------------------------------------------------------------------
// Subscriptions
// ---------------------------------------------------------------------------

func (c *Client) ListSubscriptions() ([]Subscription, error) {
	var env subscriptionsEnvelope
	if err := c.get("subscriptions", nil, &env); err != nil {
		return nil, err
	}
	return env.Subscriptions, nil
}

func (c *Client) CreateSubscription(req CreateSubscriptionRequest) (*Subscription, error) {
	body := map[string]any{
		"event": string(req.Event),
		"url":   req.TargetURL,
	}
	if req.StageSlug != "" {
		body["stage_slug"] = req.StageSlug
	}
	if req.JobShortcode != "" {
		body["job_shortcode"] = req.JobShortcode
	}
	var env subscriptionEnvelope
	if err := c.post("subscriptions", body, &env); err != nil {
		return nil, err
	}
	return &env.Subscription, nil
}

func (c *Client) DeleteSubscription(id int) error {
	return c.delete(fmt.Sprintf("subscriptions/%d", id))
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func (c *Client) get(path string, params url.Values, out any) error {
	u := c.baseURL + "/" + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	const maxRetries = 3
	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = c.http.Get(u)
		if err != nil {
			return fmt.Errorf("workableclient: GET %s: %w", path, err)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries-1 {
			retryAfter := 10
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if v, e := strconv.Atoi(ra); e == nil {
					retryAfter = v
				}
			}
			resp.Body.Close()
			time.Sleep(time.Duration(retryAfter) * time.Second)
			continue
		}
		break
	}

	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) post(path string, body map[string]any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("workableclient: marshal POST body: %w", err)
	}
	resp, err := c.http.Post(c.baseURL+"/"+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("workableclient: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/"+path, nil)
	if err != nil {
		return fmt.Errorf("workableclient: build DELETE request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("workableclient: DELETE %s: %w", path, err)
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	var msg struct {
		Message string `json:"message"`
	}
	detail := string(body)
	if err := json.Unmarshal(body, &msg); err == nil && msg.Message != "" {
		detail = msg.Message
	}
	return &APIError{StatusCode: resp.StatusCode, Message: detail}
}

// ---------------------------------------------------------------------------
// Pagination helper (generic)
// ---------------------------------------------------------------------------

// pageable is satisfied by any response envelope that carries a paging map.
type pageable interface {
	nextSinceID() string
}

func (e *jobsEnvelope) nextSinceID() string       { return extractSinceID(e.Paging) }
func (e *candidatesEnvelope) nextSinceID() string { return extractSinceID(e.Paging) }
func (e *eventsEnvelope) nextSinceID() string     { return extractSinceID(e.Paging) }
func (e *membersEnvelope) nextSinceID() string    { return extractSinceID(e.Paging) }

func extractSinceID(paging map[string]string) string {
	next, ok := paging["next"]
	if !ok || next == "" {
		return ""
	}
	u, err := url.Parse(next)
	if err != nil {
		return ""
	}
	return u.Query().Get("since_id")
}

// itemsOf extracts the typed slice from the envelope using a key name.
// We keep this simple with a type-switch rather than reflection.
func itemsOf[T any, E any](env *E, key string) []T {
	// The generic constraint doesn't let us switch on field names directly,
	// so we round-trip through JSON. This is called at most maxPages times
	// so the overhead is negligible compared to network latency.
	raw, _ := json.Marshal(env)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	val, ok := m[key]
	if !ok {
		return nil
	}
	var items []T
	_ = json.Unmarshal(val, &items)
	return items
}

// paginateList fetches all pages for list endpoints that use since_id cursors.
func paginateList[T any, E interface {
	pageable
	*EP
}, EP any](
	c *Client,
	path string,
	params url.Values,
	listKey string,
	paginate bool,
) ([]T, error) {
	var env EP
	envPtr := E(&env)
	if err := c.get(path, params, envPtr); err != nil {
		return nil, err
	}
	results := itemsOf[T, EP](&env, listKey)

	if !paginate {
		return results, nil
	}

	for page := 1; page < maxPages; page++ {
		sinceID := envPtr.nextSinceID()
		if sinceID == "" {
			break
		}
		p := url.Values{}
		for k, v := range params {
			p[k] = v
		}
		p.Set("since_id", sinceID)

		var next EP
		nextPtr := E(&next)
		if err := c.get(path, p, nextPtr); err != nil {
			return nil, err
		}
		batch := itemsOf[T, EP](&next, listKey)
		if len(batch) == 0 {
			break
		}
		results = append(results, batch...)
		env = next
		envPtr = nextPtr
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Bearer token transport
// ---------------------------------------------------------------------------

type bearerTransport struct {
	token   string
	wrapped http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	clone.Header.Set("Content-Type", "application/json")
	return t.wrapped.RoundTrip(clone)
}

// ---------------------------------------------------------------------------
// Members
// ---------------------------------------------------------------------------

func (c *Client) ListMembers(opts ListMembersOptions) ([]Member, error) {
	params := url.Values{}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	params.Set("limit", strconv.Itoa(limit))

	if opts.SinceID != "" {
		params.Set("since_id", opts.SinceID)
	}
	if opts.Email != "" {
		params.Set("email", opts.Email)
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}

	return paginateList[Member, *membersEnvelope](c, "members", params, "members", opts.Paginate)
}
