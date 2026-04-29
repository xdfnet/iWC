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
	workDir string
	cliPath string
}

// NewAgent 创建 Claude Code Agent
func NewAgent(workDir, cliPath string) *Agent {
	if cliPath == "" {
		cliPath = "claude"
	}
	log.Printf("Agent 配置: cli=%s, workDir=%s", cliPath, workDir)
	return &Agent{
		workDir: workDir,
		cliPath: cliPath,
	}
}

// SendWithSession 发送消息，指定 session（空则创建新 session）
func (a *Agent) SendWithSession(ctx context.Context, text, sessionID string) (string, string, error) {
	args := []string{"--print"}

	// 如果有 sessionID，则恢复该会话
	if strings.TrimSpace(sessionID) != "" {
		args = append(args, "--resume", sessionID)
	}

	log.Printf("📤 启动 Claude Code 处理消息 (session=%s)...", sessionID)

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

	// 尝试获取新 session ID（如果不是 resume 模式）
	newSessionID := sessionID
	if sessionID == "" {
		newID := findLatestSession()
		if newID != "" {
			newSessionID = newID
			log.Printf("💾 新会话 ID: %s", newSessionID)
		}
	}

	return final, newSessionID, nil
}

// findLatestSession 查找最新的 session 文件并返回 sessionId
func findLatestSession() string {
	sessionsDir := filepath.Join(os.Getenv("HOME"), ".claude", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return ""
	}

	// 找最新的非目录、非 .json 文件（session 文件是纯数字名）
	var newestFile string
	var newestTime int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > newestTime {
			newestTime = info.ModTime().UnixNano()
			newestFile = name
		}
	}

	if newestFile == "" {
		return ""
	}

	// 读取 JSON 获取 sessionId
	b, err := os.ReadFile(filepath.Join(sessionsDir, newestFile))
	if err != nil {
		return ""
	}
	var sess struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(b, &sess); err != nil {
		return ""
	}
	return sess.SessionID
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