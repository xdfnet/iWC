package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentSendReturnsOutput(t *testing.T) {
	cliPath := writeTestCLI(t, "cat")
	agent := NewAgent("", cliPath)

	got, err := agent.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("Send returned %q, want %q", got, "hello")
	}
}

func TestAgentSendIncludesStderrOnFailure(t *testing.T) {
	cliPath := writeTestCLI(t, "echo boom >&2\nexit 7")
	agent := NewAgent("", cliPath)

	_, err := agent.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("Send returned nil error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Send error %q does not include stderr", err)
	}
}

func TestAgentSendHonorsContextTimeout(t *testing.T) {
	cliPath := writeTestCLI(t, "while true; do :; done")
	agent := NewAgent("", cliPath)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := agent.Send(ctx, "hello")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Send error = %v, want context deadline exceeded", err)
	}
}

func writeTestCLI(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test-cli")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" != \"--print\" ]; then\n" +
		"  echo unexpected args: \"$@\" >&2\n" +
		"  exit 2\n" +
		"fi\n" +
		body + "\n"

	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write test cli: %v", err)
	}
	return path
}
