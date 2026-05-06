package claude

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
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
	args := []string{"--print", "--output-format", "json"}
	newSessionID := strings.TrimSpace(sessionID)

	// 如果有 sessionID，则恢复该会话
	if newSessionID != "" {
		args = append(args, "--resume", newSessionID)
	} else {
		generated, err := newUUID()
		if err != nil {
			return "", "", fmt.Errorf("生成 session ID 失败: %w", err)
		}
		newSessionID = generated
		args = append(args, "--session-id", newSessionID)
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

		// 如果是 session 不存在错误，清除 session 并重试（不带 --resume）
		if strings.Contains(errMsg, "No conversation found") {
			log.Printf("🔄 Session 已失效，将创建新对话 (retry)...")
			return a.sendWithoutSession(ctx, text)
		}

		return "", "", fmt.Errorf("Claude Code 错误: %s", errMsg)
	}

	final, outputSessionID, err := parsePrintJSON(output)
	if err != nil {
		return "", outputSessionID, err
	}
	if outputSessionID != "" {
		newSessionID = outputSessionID
	}
	log.Printf("✅ Claude Code 回复完成 (%d 字符, %v)", len(final), elapsed)

	if strings.TrimSpace(sessionID) == "" {
		log.Printf("💾 新会话 ID: %s", newSessionID)
	}

	return final, newSessionID, nil
}

// sendWithoutSession 不使用 session 发送消息（新建对话）
func (a *Agent) sendWithoutSession(ctx context.Context, text string) (string, string, error) {
	newSessionID, err := newUUID()
	if err != nil {
		return "", "", fmt.Errorf("生成 session ID 失败: %w", err)
	}
	args := []string{"--print", "--output-format", "json", "--session-id", newSessionID}

	log.Printf("📤 创建新 Claude Code 对话...")

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

	final, outputSessionID, err := parsePrintJSON(output)
	if err != nil {
		return "", outputSessionID, err
	}
	if outputSessionID != "" {
		newSessionID = outputSessionID
	}
	log.Printf("✅ Claude Code 回复完成 (%d 字符, %v)", len(final), elapsed)
	log.Printf("💾 新会话 ID: %s", newSessionID)

	return final, newSessionID, nil
}

func parsePrintJSON(output []byte) (string, string, error) {
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return "", "", nil
	}
	var resp struct {
		Result    string `json:"result"`
		SessionID string `json:"session_id"`
		IsError   bool   `json:"is_error"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return raw, "", nil
	}
	if resp.IsError {
		errMsg := strings.TrimSpace(resp.Error)
		if errMsg == "" {
			errMsg = strings.TrimSpace(resp.Result)
		}
		if errMsg == "" {
			errMsg = "Claude Code returned an error"
		}
		return "", strings.TrimSpace(resp.SessionID), fmt.Errorf("Claude Code 错误: %s", errMsg)
	}
	return strings.TrimSpace(resp.Result), strings.TrimSpace(resp.SessionID), nil
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
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
