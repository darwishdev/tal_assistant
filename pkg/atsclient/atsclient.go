// Package atsclient provides an HTTP client for the Mawhub ATS (ERPNext-based) API.
// Authentication is cookie-based: Login() stores the session cookie (sid) and
// all subsequent requests replay it automatically via a shared http.Client jar.
package atsclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

// ATSClientInterface defines the three ATS operations.
type ATSClientInterface interface {
	// Login authenticates against the ERPNext API and stores the session cookie.
	// Must be called before any other method.
	Login(username, password string) (*LoginResponse, error)

	// InterviewList returns all interviews visible to the current session.
	InterviewList() ([]InterviewListItem, error)

	// InterviewFind returns the full detail for a single interview by name (e.g. "HR-INT-2026-0002").
	InterviewFind(name string) (*InterviewFindResult, error)
}

// ATSClient is the concrete implementation of ATSClientInterface.
type ATSClient struct {
	baseURL string
	http    *http.Client
}

// NewATSClient creates a ready-to-use client pointed at baseURL
// (e.g. "http://192.168.100.71:8000"). Call Login() before any data methods.
func NewATSClient(baseURL string) (*ATSClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("atsclient: cookie jar: %w", err)
	}
	return &ATSClient{
		baseURL: baseURL,
		http:    &http.Client{Jar: jar},
	}, nil
}

// ── Login ──────────────────────────────────────────────────────────────────

func (c *ATSClient) Login(username, password string) (*LoginResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"usr": username,
		"pwd": password,
	})

	resp, err := c.http.Post(
		c.baseURL+"/api/method/login",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("atsclient: login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("atsclient: login: unexpected status %d", resp.StatusCode)
	}

	var out LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("atsclient: login decode: %w", err)
	}
	return &out, nil
}

// ── Interview List ─────────────────────────────────────────────────────────

func (c *ATSClient) InterviewList() ([]InterviewListItem, error) {
	resp, err := c.http.Get(c.baseURL + "/api/method/mawhub.interview_list")
	if err != nil {
		return nil, fmt.Errorf("atsclient: interview_list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("atsclient: interview_list: unexpected status %d", resp.StatusCode)
	}

	var envelope interviewListResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("atsclient: interview_list decode: %w", err)
	}
	return envelope.Message, nil
}

// ── Interview Find ─────────────────────────────────────────────────────────

func (c *ATSClient) InterviewFind(name string) (*InterviewFindResult, error) {
	endpoint := c.baseURL + "/api/method/mawhub.interview_find?name=" + url.QueryEscape(name)
	resp, err := c.http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("atsclient: interview_find request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("atsclient: interview_find: unexpected status %d", resp.StatusCode)
	}

	var envelope interviewFindResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("atsclient: interview_find decode: %w", err)
	}
	return &envelope.Message, nil
}
