package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPidFilePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "iwc", "iwc.pid")
	if got := pidFilePath(); got != want {
		t.Errorf("pidFilePath() = %q, want %q", got, want)
	}
}

func TestPlistPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "Library", "LaunchAgents", plistName)
	if got := plistPath(); got != want {
		t.Errorf("plistPath() = %q, want %q", got, want)
	}
}

func TestVersion(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}
}

func TestPrintUsage(t *testing.T) {
	// 确保 printUsage 不 panic
	printUsage()
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"world", "'world'"},
	}
	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// shellQuote 辅助函数（从 setup 相关代码复用）
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
