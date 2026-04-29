package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/admin/iCode/iCC/claude"
	"github.com/admin/iCode/iCC/weixin"
)

// --- 纯函数测试 ---

func TestShortID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"12345678901234567890", "12345678901234567890"},        // 20 chars
		{"123456789012345678901", "12345678901234567890…"},       // 21 chars
		{"12345678901234567890123456", "12345678901234567890…"}, // 26 chars
	}
	for _, tt := range tests {
		got := shortID(tt.in)
		if got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"", 5, ""},
		{"abc", 5, "abc"},
		{"abcde", 5, "abcde"},
		{"abcdef", 5, "abcde…"},
		{"hello world", 5, "hello…"},
	}
	for _, tt := range tests {
		got := truncateText(tt.in, tt.n)
		if got != tt.want {
			t.Errorf("truncateText(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}
}

func TestSplitChunks(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want int
	}{
		{"empty", "", 100, 1},
		{"short", "hello", 100, 1},
		{"exact", "hello", 5, 1},
		{"one over", "hello!", 5, 2},
		{"many chunks", "hello world test 123", 4, 5},
		{"max zero", "hello", 0, 1},
		{"max negative", "hello", -1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitChunks(tt.s, tt.max)
			if len(chunks) != tt.want {
				t.Errorf("splitChunks(%q, %d) = %d chunks, want %d", tt.s, tt.max, len(chunks), tt.want)
			}
			joined := ""
			for _, c := range chunks {
				joined += c
			}
			if joined != tt.s {
				t.Errorf("recombined = %q, want %q", joined, tt.s)
			}
		})
	}
}

func TestSplitChunksUnicode(t *testing.T) {
	chunks := splitChunks("你好世界测试消息", 3)
	expected := []string{"你好世", "界测试", "消息"}
	if len(chunks) != len(expected) {
		t.Fatalf("got %d chunks, want %d", len(chunks), len(expected))
	}
	for i, c := range chunks {
		if i < len(expected) && c != expected[i] {
			t.Errorf("chunk[%d] = %q, want %q", i, c, expected[i])
		}
	}
}

// --- Session 持久化测试 ---

func TestSessionPersistence(t *testing.T) {
	dir := t.TempDir()
	sessPath := filepath.Join(dir, "sessions.json")

	eng := New(nil, nil)
	eng.SetSessionsPath(sessPath)

	// Add sessions
	eng.sessionsMu.Lock()
	eng.sessions["user1"] = "session-abc"
	eng.sessions["user2"] = "session-xyz"
	eng.sessionsMu.Unlock()
	eng.persistSessions()

	// Verify file content
	data, err := os.ReadFile(sessPath)
	if err != nil {
		t.Fatalf("read sessions file: %v", err)
	}
	var sessions map[string]string
	if err := json.Unmarshal(data, &sessions); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sessions["user1"] != "session-abc" || sessions["user2"] != "session-xyz" {
		t.Errorf("persisted = %+v", sessions)
	}

	// Reload
	eng2 := New(nil, nil)
	eng2.SetSessionsPath(sessPath)
	eng2.loadSessions()
	eng2.sessionsMu.RLock()
	if eng2.sessions["user1"] != "session-abc" {
		t.Errorf("reloaded user1 = %q", eng2.sessions["user1"])
	}
	if eng2.sessions["user2"] != "session-xyz" {
		t.Errorf("reloaded user2 = %q", eng2.sessions["user2"])
	}
	eng2.sessionsMu.RUnlock()
}

func TestLoadSessionsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	sessPath := filepath.Join(dir, "sessions.json")
	os.WriteFile(sessPath, []byte("{}"), 0600)

	eng := New(nil, nil)
	eng.SetSessionsPath(sessPath)
	eng.loadSessions()

	eng.sessionsMu.RLock()
	if len(eng.sessions) != 0 {
		t.Errorf("expected 0, got %d", len(eng.sessions))
	}
	eng.sessionsMu.RUnlock()
}

func TestLoadSessionsNoFile(t *testing.T) {
	eng := New(nil, nil)
	eng.SetSessionsPath(filepath.Join(t.TempDir(), "nonexistent.json"))
	eng.loadSessions()

	eng.sessionsMu.RLock()
	if len(eng.sessions) != 0 {
		t.Errorf("expected empty, got %d", len(eng.sessions))
	}
	eng.sessionsMu.RUnlock()
}

func TestLoadSessionsMalformed(t *testing.T) {
	dir := t.TempDir()
	sessPath := filepath.Join(dir, "sessions.json")
	os.WriteFile(sessPath, []byte("not json"), 0600)

	eng := New(nil, nil)
	eng.SetSessionsPath(sessPath)
	eng.loadSessions()

	eng.sessionsMu.RLock()
	if len(eng.sessions) != 0 {
		t.Errorf("expected empty after malformed, got %d", len(eng.sessions))
	}
	eng.sessionsMu.RUnlock()
}

func TestPersistSessionsNoPath(t *testing.T) {
	eng := New(nil, nil)
	eng.sessionsMu.Lock()
	eng.sessions["user1"] = "session-1"
	eng.sessionsMu.Unlock()
	eng.persistSessions() // should not panic
}

// --- Engine 生命周期 ---

func TestEngineNew(t *testing.T) {
	wx := weixin.NewPlatform("token", "", "", 0, "")
	agent := claude.NewAgent("/tmp", "claude")
	eng := New(wx, agent)

	if eng.wechat != wx {
		t.Error("wechat not set")
	}
	if eng.agent != agent {
		t.Error("agent not set")
	}
	if eng.sessions == nil {
		t.Error("sessions map not initialized")
	}
}

func TestSetSessionsPath(t *testing.T) {
	eng := New(nil, nil)
	path := "/tmp/test-sessions.json"
	eng.SetSessionsPath(path)
	if eng.sessPath != path {
		t.Errorf("sessPath = %q, want %q", eng.sessPath, path)
	}
}

// --- sendToUser 边界条件 ---

func TestSendToUserEdgeCases(t *testing.T) {
	wx := weixin.NewPlatform("token", "", "", 0, t.TempDir())
	eng := New(wx, nil)

	// These should not panic even without context
	eng.sendToUser("", "hello")   // empty user
	eng.sendToUser("user1", "")   // empty content
	eng.sendToUser("", "")        // both empty

	// Send to user without context_token should fail gracefully
	eng.sendToUser("user1", "hello")
}
