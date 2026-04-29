package claude

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// Agent 管理 Claude Code 进程（每次对话启动新进程）
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

// Send 发送消息给 Claude Code 并等待回复（阻塞）
func (a *Agent) Send(ctx context.Context, text string) (string, error) {
	args := []string{"--print"}

	log.Printf("📤 启动 Claude Code 处理消息...")

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
			return "", ctxErr
		}

		errMsg := err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				errMsg = fmt.Sprintf("%s: %s", errMsg, stderr)
			}
		}
		log.Printf("❌ Claude Code 错误 (%v): %s", elapsed, errMsg)
		return "", fmt.Errorf("Claude Code 错误: %s", errMsg)
	}

	final := strings.TrimSpace(string(output))
	log.Printf("✅ Claude Code 回复完成 (%d 字符, %v)", len(final), elapsed)
	return final, nil
}

// IsRunning 始终返回 true（无状态进程）
func (a *Agent) IsRunning() bool {
	return true
}
