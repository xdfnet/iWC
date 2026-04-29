package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSendWithSessionNoResume(t *testing.T) {
	cliPath := writeTestCLI(t, "#!/bin/sh\ncat\n")
	agent := NewAgent("", cliPath)

	got, sessID, err := agent.SendWithSession(context.Background(), "hello", "")
	if err != nil {
		t.Fatalf("SendWithSession returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
	if sessID != "" {
		t.Fatalf("sessionID = %q, want empty (no session dir)", sessID)
	}
}

func TestSendWithSessionWithResume(t *testing.T) {
	cliPath := writeTestCLI(t, "#!/bin/sh\ncat\n")
	agent := NewAgent("", cliPath)

	// With session ID, should add --resume flag
	_, _, err := agent.SendWithSession(context.Background(), "hello", "some-session-id")
	if err != nil {
		t.Fatalf("SendWithSession returned error: %v", err)
	}
}

func TestSendSimple(t *testing.T) {
	cliPath := writeTestCLI(t, "#!/bin/sh\ncat\n")
	agent := NewAgent("", cliPath)

	got, err := agent.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func writeTestCLI(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test-cli")
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatalf("write test cli: %v", err)
	}
	return path
}