package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.WeChat.BaseURL != "https://ilinkai.weixin.qq.com" {
		t.Errorf("BaseURL = %q", cfg.WeChat.BaseURL)
	}
	if cfg.WeChat.LongPollMS != 35000 {
		t.Errorf("LongPollMS = %d", cfg.WeChat.LongPollMS)
	}
	if cfg.Claude.CLIPath != "claude" {
		t.Errorf("CLIPath = %q", cfg.Claude.CLIPath)
	}
	home, _ := os.UserHomeDir()
	if cfg.Claude.WorkDir != home {
		t.Errorf("WorkDir = %q, want %q", cfg.Claude.WorkDir, home)
	}
	if cfg.System.DataDir != filepath.Join(home, ".icc") {
		t.Errorf("DataDir = %q", cfg.System.DataDir)
	}
}

func TestConfigPath(t *testing.T) {
	t.Setenv("ICC_CONFIG", "")
	path := ConfigPath()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".icc", "config.toml")
	if path != want {
		t.Errorf("ConfigPath = %q, want %q", path, want)
	}
}

func TestConfigPathEnvOverride(t *testing.T) {
	t.Setenv("ICC_CONFIG", "/tmp/custom-config.toml")
	path := ConfigPath()
	if path != "/tmp/custom-config.toml" {
		t.Errorf("ConfigPath = %q, want /tmp/custom-config.toml", path)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := DefaultConfig()
	cfg.WeChat.Token = "test-token"
	cfg.WeChat.BaseURL = "https://example.com"
	cfg.WeChat.AllowFrom = "user1"
	cfg.WeChat.LongPollMS = 30000
	cfg.Claude.WorkDir = "/tmp"
	cfg.Claude.CLIPath = "/usr/local/bin/claude"
	cfg.System.DataDir = "/tmp/.icc"

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.WeChat.Token != "test-token" {
		t.Errorf("Token = %q", loaded.WeChat.Token)
	}
	if loaded.WeChat.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q", loaded.WeChat.BaseURL)
	}
	if loaded.WeChat.AllowFrom != "user1" {
		t.Errorf("AllowFrom = %q", loaded.WeChat.AllowFrom)
	}
	if loaded.WeChat.LongPollMS != 30000 {
		t.Errorf("LongPollMS = %d", loaded.WeChat.LongPollMS)
	}
	if loaded.Claude.WorkDir != "/tmp" {
		t.Errorf("WorkDir = %q", loaded.Claude.WorkDir)
	}
	if loaded.Claude.CLIPath != "/usr/local/bin/claude" {
		t.Errorf("CLIPath = %q", loaded.Claude.CLIPath)
	}
	if loaded.System.DataDir != "/tmp/.icc" {
		t.Errorf("DataDir = %q", loaded.System.DataDir)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte("not valid {{{ toml"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed config")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "path")
	path := filepath.Join(dir, "config.toml")

	cfg := DefaultConfig()
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestSaveWithEmptyPath(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	t.Setenv("ICC_CONFIG", filepath.Join(dir, "config.toml"))

	if err := Save(cfg, ""); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
}

func TestLoadUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(`
[wechat]
token = "x"
base_url = "https://x.com"

[claude]
work_dir = "/x"

[unknown_section]
foo = "bar"
`), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load with unknown fields should not error: %v", err)
	}
	if cfg.WeChat.Token != "x" {
		t.Errorf("Token = %q", cfg.WeChat.Token)
	}
}
