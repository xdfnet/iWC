package weixin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformPersistsSyncBuf(t *testing.T) {
	dataDir := t.TempDir()
	p := NewPlatform("token", "", "", 0, dataDir)

	p.syncBufMu.Lock()
	p.persistSyncBuf("cursor-1")
	p.syncBufMu.Unlock()

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
