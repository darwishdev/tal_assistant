package workableclient

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"testing"
	"tal_assistant/config"
)

type dumpingTransport struct {
	Transport http.RoundTripper
	TestName  string
}

func (t *dumpingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, _ := httputil.DumpRequestOut(req, true)

	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	respDump, _ := httputil.DumpResponse(resp, true)

	dumpContent := fmt.Sprintf("===== REQUEST =====\n%s\n\n===== RESPONSE =====\n%s\n", reqDump, respDump)

	dir := "testdata"
	os.MkdirAll(dir, 0755)
	filename := filepath.Join(dir, t.TestName+".txt")
	os.WriteFile(filename, []byte(dumpContent), 0644)

	return resp, nil
}

func TestListEvents(t *testing.T) {
	// Load config from the workspace to get WORKABLE credentials
	cfg := config.Load()
	
	subdomain := cfg.WorkableSubdomain
	token := cfg.WorkableToken

	if subdomain == "" || token == "" {
		t.Skip("Skipping real API test: WORKABLE_SUBDOMAIN and WORKABLE_TOKEN are not set in config.")
	}

	c, err := New(subdomain, token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Wrap the client's internal transport with our dumping transport
	concreteClient := c.(*Client)
	if bt, ok := concreteClient.http.Transport.(*bearerTransport); ok {
		bt.wrapped = &dumpingTransport{
			Transport: bt.wrapped,
			TestName:  "TestListEvents_Real",
		}
	} else {
		t.Fatalf("Expected bearerTransport but got %T", concreteClient.http.Transport)
	}

	// Make the real API call
	opts := ListEventsOptions{
		Limit:    10,
		MemberID: "1c3f84",
	}

	events, err := c.ListEvents(opts)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}

	if len(events) == 0 {
		t.Log("No events returned from the real API (this is fine if the account has no events).")
	} else {
		t.Logf("Successfully received %d events from the real Workable API", len(events))
		for i, ev := range events {
			t.Logf("[%d] Event ID: %s, Title: %s, Type: %s, StartsAt: %s", i, ev.ID, ev.Title, ev.Type, ev.StartsAt)
		}
	}
}

func TestListFutureEvents(t *testing.T) {
	// Load config from the workspace to get WORKABLE credentials
	cfg := config.Load()
	
	subdomain := cfg.WorkableSubdomain
	token := cfg.WorkableToken

	if subdomain == "" || token == "" {
		t.Skip("Skipping real API test: WORKABLE_SUBDOMAIN and WORKABLE_TOKEN are not set in config.")
	}

	c, err := New(subdomain, token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Wrap the client's internal transport with our dumping transport
	concreteClient := c.(*Client)
	if bt, ok := concreteClient.http.Transport.(*bearerTransport); ok {
		bt.wrapped = &dumpingTransport{
			Transport: bt.wrapped,
			TestName:  "TestListFutureEvents_Real",
		}
	} else {
		t.Fatalf("Expected bearerTransport but got %T", concreteClient.http.Transport)
	}

	// Make the real API call
	opts := ListEventsOptions{
		Limit:    10,
		MemberID: "1c3f84",
	}

	events, err := c.ListFutureEvents(opts)
	if err != nil {
		t.Fatalf("ListFutureEvents failed: %v", err)
	}

	if len(events) == 0 {
		t.Log("No future events returned from the real API.")
	} else {
		t.Logf("Successfully received %d FUTURE events from the real Workable API", len(events))
		for i, ev := range events {
			t.Logf("[%d] Event ID: %s, Title: %s, Type: %s, StartsAt: %s, EndsAt: %s", i, ev.ID, ev.Title, ev.Type, ev.StartsAt, ev.EndsAt)
		}
	}
}

func TestListMembers(t *testing.T) {
	// Load config from the workspace to get WORKABLE credentials
	cfg := config.Load()
	
	subdomain := cfg.WorkableSubdomain
	token := cfg.WorkableToken

	if subdomain == "" || token == "" {
		t.Skip("Skipping real API test: WORKABLE_SUBDOMAIN and WORKABLE_TOKEN are not set in config.")
	}

	c, err := New(subdomain, token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Wrap the client's internal transport with our dumping transport
	concreteClient := c.(*Client)
	if bt, ok := concreteClient.http.Transport.(*bearerTransport); ok {
		bt.wrapped = &dumpingTransport{
			Transport: bt.wrapped,
			TestName:  "TestListMembers_Real",
		}
	} else {
		t.Fatalf("Expected bearerTransport but got %T", concreteClient.http.Transport)
	}

	// Make the real API call
	opts := ListMembersOptions{
		Limit:  50,
		Email:  "esraa@mawhub.io",
		Status: "active",
	}

	members, err := c.ListMembers(opts)
	if err != nil {
		t.Fatalf("ListMembers failed: %v", err)
	}

	if len(members) == 0 {
		t.Log("No members returned from the real API.")
	} else {
		t.Logf("Successfully received %d members from the real Workable API", len(members))
		for i, mem := range members {
			t.Logf("[%d] Member ID: %s, Name: %s, Email: %s, Role: %s, Active: %v", i, mem.ID, mem.Name, mem.Email, mem.Role, mem.Active)
		}
	}
}
