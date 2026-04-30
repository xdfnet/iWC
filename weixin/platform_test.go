package weixin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractText(t *testing.T) {
	tests := []struct {
		name  string
		items []messageItem
		want  string
	}{
		{"empty", nil, ""},
		{"text item", []messageItem{
			{Type: messageItemText, TextItem: &textItem{Text: "hello"}},
		}, "hello"},
		{"voice item", []messageItem{
			{Type: messageItemVoice, VoiceItem: &voiceItem{Text: "voice content"}},
		}, "[语音] voice content"},
		{"first text wins", []messageItem{
			{Type: messageItemText, TextItem: &textItem{Text: "first"}},
			{Type: messageItemText, TextItem: &textItem{Text: "second"}},
		}, "first"},
		{"text before voice", []messageItem{
			{Type: messageItemText, TextItem: &textItem{Text: "text msg"}},
			{Type: messageItemVoice, VoiceItem: &voiceItem{Text: "voice msg"}},
		}, "text msg"},
		{"voice only", []messageItem{
			{Type: messageItemVoice, VoiceItem: &voiceItem{Text: "hello"}},
		}, "[语音] hello"},
		{"unknown type skipped", []messageItem{
			{Type: 99},
			{Type: messageItemText, TextItem: &textItem{Text: "after unknown"}},
		}, "after unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.items)
			if got != tt.want {
				t.Errorf("extractText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAllowList(t *testing.T) {
	tests := []struct {
		name      string
		allowFrom string
		userID    string
		want      bool
	}{
		{"empty allows all", "", "anyone", true},
		{"star allows all", "*", "anyone", true},
		{"exact match", "user1", "user1", true},
		{"case insensitive", "User1", "user1", true},
		{"no match", "user1", "user2", false},
		{"multi match first", "user1,user2,user3", "user2", true},
		{"multi match last", "user1,user2", "user2", true},
		{"multi no match", "user1,user2", "user3", false},
		{"whitespace in list", " user1 , user2 ", "user2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allowList(tt.allowFrom, tt.userID)
			if got != tt.want {
				t.Errorf("allowList(%q, %q) = %v, want %v", tt.allowFrom, tt.userID, got, tt.want)
			}
		})
	}
}

func TestSplitRunes(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want int // number of chunks
	}{
		{"empty", "", 10, 1},
		{"shorter than max", "abc", 10, 1},
		{"exact boundary", "abcde", 5, 1},
		{"one rune over", "abcdef", 5, 2},
		{"multi chunk", "hello world test", 5, 4},
		{"unicode", "你好世界测试", 3, 2},
		{"max zero", "hello", 0, 1},
		{"max negative", "hello", -1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitRunes(tt.s, tt.max)
			if len(chunks) != tt.want {
				t.Errorf("splitRunes(%q, %d) = %d chunks, want %d", tt.s, tt.max, len(chunks), tt.want)
			}
			// Verify recombination
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

func TestSetAndGetContextToken(t *testing.T) {
	dataDir := t.TempDir()
	p := NewPlatform("token", "", "", 0, dataDir)

	// Initially empty
	if got := p.getContextToken("user1"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// Set and get
	p.setContextToken("user1", "token-abc")
	if got := p.getContextToken("user1"); got != "token-abc" {
		t.Errorf("got %q, want token-abc", got)
	}

	// Empty userID or token should be no-op
	p.setContextToken("", "token")
	if got := p.getContextToken(""); got != "" {
		t.Errorf("empty userID should not set, got %q", got)
	}
	p.setContextToken("user2", "")
	if got := p.getContextToken("user2"); got != "" {
		t.Errorf("empty token should not set, got %q", got)
	}

	// Persistence check
	reloaded := NewPlatform("token", "", "", 0, dataDir)
	if got := reloaded.getContextToken("user1"); got != "token-abc" {
		t.Errorf("reloaded token = %q, want token-abc", got)
	}
}

func TestHandleMessage(t *testing.T) {
	p := NewPlatform("token", "", "", 0, t.TempDir())

	t.Run("filters bot messages", func(t *testing.T) {
		var received *IncomingMessage
		p.Start(func(msg *IncomingMessage) { received = msg })
		defer p.Stop()

		m := &weixinMessage{
			MessageType: messageTypeBot,
			FromUserID:  "user1",
			MessageID:   1,
			ItemList:    []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "bot msg"}}},
		}
		p.handleMessage(m, p.handler)
		if received != nil {
			t.Error("bot messages should be filtered")
		}
	})

	t.Run("filters unknown message types", func(t *testing.T) {
		var received *IncomingMessage
		p := NewPlatform("token", "", "", 0, t.TempDir())
		p.Start(func(msg *IncomingMessage) { received = msg })
		defer p.Stop()

		m := &weixinMessage{
			MessageType: 99, // unknown
			FromUserID:  "user1",
			MessageID:   1,
			ItemList:    []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "test"}}},
		}
		p.handleMessage(m, p.handler)
		if received != nil {
			t.Error("unknown message types should be filtered")
		}
	})

	t.Run("filters empty from_user_id", func(t *testing.T) {
		var received *IncomingMessage
		p := NewPlatform("token", "", "", 0, t.TempDir())
		p.Start(func(msg *IncomingMessage) { received = msg })
		defer p.Stop()

		m := &weixinMessage{
			MessageType: messageTypeUser,
			FromUserID:  "",
			MessageID:   1,
			ItemList:    []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "test"}}},
		}
		p.handleMessage(m, p.handler)
		if received != nil {
			t.Error("empty from_user_id should be filtered")
		}
	})

	t.Run("respects allow_from", func(t *testing.T) {
		var received *IncomingMessage
		p := NewPlatform("token", "", "allowed-user", 0, t.TempDir())
		p.Start(func(msg *IncomingMessage) { received = msg })
		defer p.Stop()

		m := &weixinMessage{
			MessageType:  messageTypeUser,
			FromUserID:   "blocked-user",
			MessageID:    1,
			ContextToken: "ctx",
			ItemList:     []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "test"}}},
		}
		p.handleMessage(m, p.handler)
		if received != nil {
			t.Error("blocked user should be filtered")
		}
	})

	t.Run("saves context_token", func(t *testing.T) {
		var received *IncomingMessage
		p := NewPlatform("token", "", "", 0, t.TempDir())
		p.Start(func(msg *IncomingMessage) { received = msg })
		defer p.Stop()

		m := &weixinMessage{
			MessageType:  messageTypeUser,
			FromUserID:   "user1",
			MessageID:    1,
			ContextToken: "new-ctx-token",
			ItemList:     []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "hello"}}},
		}
		p.handleMessage(m, p.handler)

		if received == nil {
			t.Fatal("message should be received")
		}
		if received.ContextToken != "new-ctx-token" {
			t.Errorf("ContextToken = %q", received.ContextToken)
		}
		if p.getContextToken("user1") != "new-ctx-token" {
			t.Error("context_token should be saved")
		}
	})

	t.Run("deduplicates messages", func(t *testing.T) {
		count := 0
		p := NewPlatform("token", "", "", 0, t.TempDir())
		p.Start(func(msg *IncomingMessage) { count++ })
		defer p.Stop()

		m := &weixinMessage{
			MessageType:  messageTypeUser,
			FromUserID:   "user1",
			MessageID:    42,
			CreateTimeMs: 1000,
			ContextToken: "ctx",
			ItemList:     []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "dup"}}},
		}
		p.handleMessage(m, p.handler)
		p.handleMessage(m, p.handler) // duplicate

		if count != 1 {
			t.Errorf("got %d messages, want 1 (dedup)", count)
		}
	})
}

func TestPlatformStartStop(t *testing.T) {
	p := NewPlatform("token", "", "", 0, t.TempDir())

	// Start
	err := p.Start(func(msg *IncomingMessage) {})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Second Start should be no-op
	err = p.Start(func(msg *IncomingMessage) {})
	if err != nil {
		t.Fatalf("second Start should be no-op: %v", err)
	}

	// Stop
	p.Stop()

	// Stop again should be safe
	p.Stop()
}

func TestPlatformPersistsSyncBuf(t *testing.T) {
	dataDir := t.TempDir()
	p := NewPlatform("token", "", "", 0, dataDir)

	p.persistSyncBuf("cursor-1")

	path := filepath.Join(dataDir, "wechat", "get_updates.buf")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sync buf: %v", err)
	}
	if string(got) != "cursor-1" {
		t.Fatalf("sync buf = %q, want cursor-1", got)
	}

	reloaded := NewPlatform("token", "", "", 0, dataDir)
	reloaded.syncBufMu.Lock()
	defer reloaded.syncBufMu.Unlock()
	if reloaded.syncBuf != "cursor-1" {
		t.Fatalf("reloaded sync buf = %q, want cursor-1", reloaded.syncBuf)
	}
}

func TestPlatformPersistsContextTokens(t *testing.T) {
	dataDir := t.TempDir()
	p := NewPlatform("token", "", "", 0, dataDir)

	p.setContextToken("user@im.wechat", "ctx-token")

	path := filepath.Join(dataDir, "wechat", "context_tokens.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("context token cache missing: %v", err)
	}

	reloaded := NewPlatform("token", "", "", 0, dataDir)
	reloaded.tokensMu.RLock()
	got := reloaded.tokens["user@im.wechat"]
	reloaded.tokensMu.RUnlock()
	if got != "ctx-token" {
		t.Fatalf("reloaded context token = %q, want ctx-token", got)
	}
}

func TestStartTypingLifecycle(t *testing.T) {
	statusCh := make(chan int, 4)
	userID := "user@im.wechat"
	contextToken := "ctx-token"
	ticket := "typing-ticket"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			var req getConfigReq
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode getconfig: %v", err)
				return
			}
			if req.UserID != userID || req.ContextToken != contextToken {
				t.Errorf("getconfig request = %+v", req)
			}
			_ = json.NewEncoder(w).Encode(getConfigResp{TypingTicket: ticket})
		case "/ilink/bot/sendtyping":
			var req sendTypingReq
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode sendtyping: %v", err)
				return
			}
			if req.IlinkUserID != userID || req.TypingTicket != ticket {
				t.Errorf("sendtyping request = %+v", req)
			}
			statusCh <- req.Status
			_ = json.NewEncoder(w).Encode(sendMessageResp{})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := NewPlatform("token", server.URL, "", 0, t.TempDir())
	p.SetContextTokenForTest(userID, contextToken)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := p.StartTyping(ctx, userID)

	if got := waitTypingStatus(t, statusCh); got != typingStatusStart {
		t.Fatalf("first typing status = %d, want start", got)
	}
	stop()
	if got := waitTypingStatus(t, statusCh); got != typingStatusStop {
		t.Fatalf("second typing status = %d, want stop", got)
	}
}

func waitTypingStatus(t *testing.T, ch <-chan int) int {
	t.Helper()
	select {
	case status := <-ch:
		return status
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for typing status")
	}
	return 0
}
