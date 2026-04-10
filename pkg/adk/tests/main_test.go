package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tal_assistant/pkg/adk"
	"testing"
	"time"
)

var testUserName = "test_user"

func newService(t *testing.T) adk.ADKServiceInterface {
	t.Helper()
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		t.Skip("GOOGLE_API_KEY not set — skipping integration test")
	}
	ctx := context.Background()
	svc, err := adk.NewADKService(ctx, key)
	if err != nil {
		t.Fatalf("NewADKService: %v", err)
	}
	return svc
}

func writeRecord(t *testing.T, sessionID, input, output string) {
	t.Helper()
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	name := strings.ReplaceAll(t.Name(), "/", "_")
	path := filepath.Join("testdata", name+".txt")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open record file: %v", err)
	}
	defer f.Close()
	fmt.Fprintf(f, "────────────────────────────────────────\n")
	fmt.Fprintf(f, "test      : %s\n", t.Name())
	fmt.Fprintf(f, "timestamp : %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "session   : %s\n", sessionID)
	fmt.Fprintf(f, "input     : %s\n", input)
	fmt.Fprintf(f, "output    : %s\n", output)
	fmt.Fprintf(f, "\n")
}

func uniqueID(t *testing.T) string {
	return strings.ReplaceAll(t.Name(), "/", "_") +
		"_" + fmt.Sprintf("%d", time.Now().UnixMilli())
}
