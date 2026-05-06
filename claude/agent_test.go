package claude

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSendWithSessionNoResume(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.txt")
	cliPath := writeTestCLI(t, "#!/bin/sh\nprintf '%s\\n' \"$@\" > "+shellQuote(argsPath)+"\ncat >/dev/null\nprintf '{\"result\":\"hello\",\"session_id\":\"json-session\"}'\n")
	agent := NewAgent("", cliPath)

	got, sessID, err := agent.SendWithSession(context.Background(), "hello", "")
	if err != nil {
		t.Fatalf("SendWithSession returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
	if sessID != "json-session" {
		t.Fatalf("sessionID = %q, want json-session", sessID)
	}
	args := readFile(t, argsPath)
	if !strings.Contains(args, "--output-format\njson\n") {
		t.Fatalf("args missing json output format: %q", args)
	}
	if !strings.Contains(args, "--session-id\n") {
		t.Fatalf("args missing --session-id: %q", args)
	}
	if strings.Contains(args, "--resume\n") {
		t.Fatalf("args should not contain --resume: %q", args)
	}
}

func TestSendWithSessionWithResume(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.txt")
	cliPath := writeTestCLI(t, "#!/bin/sh\nprintf '%s\\n' \"$@\" > "+shellQuote(argsPath)+"\ncat >/dev/null\nprintf '{\"result\":\"hello\",\"session_id\":\"some-session-id\"}'\n")
	agent := NewAgent("", cliPath)

	got, sessID, err := agent.SendWithSession(context.Background(), "hello", "some-session-id")
	if err != nil {
		t.Fatalf("SendWithSession returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
	if sessID != "some-session-id" {
		t.Fatalf("sessionID = %q, want some-session-id", sessID)
	}
	args := readFile(t, argsPath)
	if !strings.Contains(args, "--resume\nsome-session-id\n") {
		t.Fatalf("args missing resume session: %q", args)
	}
	if strings.Contains(args, "--session-id\n") {
		t.Fatalf("resume args should not contain --session-id: %q", args)
	}
}

func TestSendWithSessionStaleResumeRetriesNewSession(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.txt")
	argsPath := filepath.Join(dir, "args.txt")
	cliPath := writeTestCLI(t, "#!/bin/sh\ncount=0\nif [ -f "+shellQuote(statePath)+" ]; then count=$(cat "+shellQuote(statePath)+"); fi\ncount=$((count + 1))\necho \"$count\" > "+shellQuote(statePath)+"\necho \"call:$count\" >> "+shellQuote(argsPath)+"\nprintf '%s\\n' \"$@\" >> "+shellQuote(argsPath)+"\ncat >/dev/null\ncase \" $* \" in\n  *\" --resume stale-session \"*) echo 'No conversation found' >&2; exit 1 ;;\nesac\nprintf '{\"result\":\"fresh\",\"session_id\":\"fresh-session\"}'\n")
	agent := NewAgent("", cliPath)

	got, sessID, err := agent.SendWithSession(context.Background(), "hello", "stale-session")
	if err != nil {
		t.Fatalf("SendWithSession returned error: %v", err)
	}
	if got != "fresh" {
		t.Fatalf("got %q, want fresh", got)
	}
	if sessID != "fresh-session" {
		t.Fatalf("sessionID = %q, want fresh-session", sessID)
	}
	args := readFile(t, argsPath)
	if !strings.Contains(args, "--resume\nstale-session\n") {
		t.Fatalf("first call did not resume stale session: %q", args)
	}
	if !strings.Contains(args, "call:2\n") || !strings.Contains(args, "--session-id\n") {
		t.Fatalf("retry did not create a new session: %q", args)
	}
}

func TestSendSimple(t *testing.T) {
	cliPath := writeTestCLI(t, "#!/bin/sh\ncat >/dev/null\nprintf '{\"result\":\"hello\",\"session_id\":\"simple-session\"}'\n")
	agent := NewAgent("", cliPath)

	got, err := agent.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestSendWithSessionJSONError(t *testing.T) {
	cliPath := writeTestCLI(t, "#!/bin/sh\ncat >/dev/null\nprintf '{\"is_error\":true,\"error\":\"permission denied\",\"session_id\":\"error-session\"}'\n")
	agent := NewAgent("", cliPath)

	got, sessID, err := agent.SendWithSession(context.Background(), "hello", "")
	if err == nil {
		t.Fatal("SendWithSession returned nil error")
	}
	if got != "" {
		t.Fatalf("got %q, want empty response", got)
	}
	if sessID != "error-session" {
		t.Fatalf("sessionID = %q, want error-session", sessID)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error = %q, want permission denied", err)
	}
}

func TestNewUUIDFormat(t *testing.T) {
	id, err := newUUID()
	if err != nil {
		t.Fatalf("newUUID returned error: %v", err)
	}
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(id) {
		t.Fatalf("uuid = %q, want RFC 4122 v4 format", id)
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
