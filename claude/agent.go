package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Agent 管理 Claude Code 进程
type Agent struct {
	workDir     string
	cliPath     string
	sessionsDir string
}

// NewAgent 创建 Claude Code Agent
func NewAgent(workDir, cliPath string) *Agent {
	if cliPath == "" {
		cliPath = "claude"
	}
	log.Printf("Agent 配置: cli=%s, workDir=%s", cliPath, workDir)
	return &Agent{
		workDir:     workDir,
		cliPath:     cliPath,
		sessionsDir: filepath.Join(os.Getenv("HOME"), ".claude", "sessions"),
	}
}

// SetSessionsDir 设置 sessions 目录（测试用）
func (a *Agent) SetSessionsDir(dir string) {
	a.sessionsDir = dir
}

// SendWithSession 发送消息，指定 session（空则创建新 session）
func (a *Agent) SendWithSession(ctx context.Context, text, sessionID string) (string, string, error) {
	args := []string{"--print"}

	// 如果有 sessionID，则恢复该会话
	if strings.TrimSpace(sessionID) != "" {
		args = append(args, "--resume", sessionID)
	}

	log.Printf("📤 启动 Claude Code 处理消息 (session=%s)...", sessionID)

	// 快照当前 sessions 文件集（用于新建场景下精确匹配新 session）
	var snap map[string]bool
	if sessionID == "" {
		snap = a.snapshotSessions()
	}

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	if strings.TrimSpace(a.workDir) != "" {
		cmd.Dir = a.workDir
	}
	cmd.Stdin = strings.NewReader(text)

	start := time.Now()
	output, err := cmd.Output()
	elapsed := time.Since(start)

	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Printf("❌ Claude Code 已停止 (%v): %v", elapsed, ctxErr)
			return "", "", ctxErr
		}

		errMsg := err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				errMsg = fmt.Sprintf("%s: %s", errMsg, stderr)
			}
		}
		log.Printf("❌ Claude Code 错误 (%v): %s", elapsed, errMsg)

		// 如果是 session 不存在错误，清除 session 并重试（不带 --resume）
		if strings.Contains(errMsg, "No conversation found") {
			log.Printf("🔄 Session 已失效，将创建新对话 (retry)...")
			// 重试，不带 session
			return a.sendWithoutSession(ctx, text)
		}

		return "", "", fmt.Errorf("Claude Code 错误: %s", errMsg)
	}

	final := strings.TrimSpace(string(output))
	log.Printf("✅ Claude Code 回复完成 (%d 字符, %v)", len(final), elapsed)

	// 尝试获取新 session ID（只在新建对话时）
	newSessionID := sessionID
	if sessionID == "" {
		newID := a.findNewSession(snap)
		if newID != "" {
			newSessionID = newID
			log.Printf("💾 新会话 ID: %s", newSessionID)
		}
	}

	return final, newSessionID, nil
}

// snapshotSessions 返回当前 sessions 目录下有效 session 的文件名集合
func (a *Agent) snapshotSessions() map[string]bool {
	snap := make(map[string]bool)
	entries, err := os.ReadDir(a.sessionsDir)
	if err != nil {
		return snap
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.sessionsDir, name))
		if err != nil {
			continue
		}
		var sess struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(b, &sess); err != nil || sess.SessionID == "" {
			continue
		}
		snap[name] = true
	}
	return snap
}

// findNewSession 在快照之后查找新出现的 session 文件，返回其 sessionId
func (a *Agent) findNewSession(snap map[string]bool) string {
	if snap == nil {
		return ""
	}
	entries, err := os.ReadDir(a.sessionsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") || snap[name] {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.sessionsDir, name))
		if err != nil {
			continue
		}
		var sess struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(b, &sess); err != nil || sess.SessionID == "" {
			continue
		}
		return sess.SessionID
	}
	return ""
}

// sendWithoutSession 不使用 session 发送消息（新建对话）
func (a *Agent) sendWithoutSession(ctx context.Context, text string) (string, string, error) {
	args := []string{"--print"}

	log.Printf("📤 创建新 Claude Code 对话...")

	// 快照当前 sessions 文件集（用于精确匹配新建的 session）
	snap := a.snapshotSessions()

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	if strings.TrimSpace(a.workDir) != "" {
		cmd.Dir = a.workDir
	}
	cmd.Stdin = strings.NewReader(text)

	start := time.Now()
	output, err := cmd.Output()
	elapsed := time.Since(start)

	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Printf("❌ Claude Code 已停止 (%v): %v", elapsed, ctxErr)
			return "", "", ctxErr
		}
		errMsg := err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				errMsg = fmt.Sprintf("%s: %s", errMsg, stderr)
			}
		}
		log.Printf("❌ Claude Code 错误 (%v): %s", elapsed, errMsg)
		return "", "", fmt.Errorf("Claude Code 错误: %s", errMsg)
	}

	final := strings.TrimSpace(string(output))
	log.Printf("✅ Claude Code 回复完成 (%d 字符, %v)", len(final), elapsed)

	// 获取新 session ID
	newID := a.findNewSession(snap)
	if newID != "" {
		log.Printf("💾 新会话 ID: %s", newID)
	}
	return final, newID, nil
}

// Send 发送消息（无 session 复用，每次新建）
func (a *Agent) Send(ctx context.Context, text string) (string, error) {
	out, _, err := a.SendWithSession(ctx, text, "")
	return out, err
}

// IsRunning 始终返回 true（无状态进程）
func (a *Agent) IsRunning() bool {
	return true
}