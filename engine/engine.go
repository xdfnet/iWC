package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/admin/iCode/iCC/claude"
	"github.com/admin/iCode/iCC/weixin"
)

const maxSendLen = 3800
const agentTimeout = 60 * time.Second

// Engine 桥接 WeChat 消息和 Claude Code
type Engine struct {
	wechat *weixin.Platform
	agent  *claude.Agent
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New 创建引擎
func New(wechat *weixin.Platform, agent *claude.Agent) *Engine {
	return &Engine{
		wechat: wechat,
		agent:  agent,
	}
}

// Start 启动引擎
func (e *Engine) Start(ctx context.Context) error {
	e.ctx, e.cancel = context.WithCancel(ctx)

	// 启动微信消息监听
	if err := e.wechat.Start(e.handleWeChatMessage); err != nil {
		return fmt.Errorf("启动微信平台失败: %w", err)
	}

	log.Println("✅ iCC 引擎已启动")
	log.Println("   Claude Code 就绪，等待微信消息...")

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wechat.Stop()
	e.wg.Wait()
	log.Println("iCC 引擎已停止")
}

// handleWeChatMessage 处理来自微信的消息
func (e *Engine) handleWeChatMessage(msg *weixin.IncomingMessage) {
	log.Printf("📩 收到微信消息 [来自: %s]: %s", shortID(msg.FromUserID), truncateText(msg.Content, 50))

	// 后台处理，不阻塞轮询
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.processMessage(msg)
	}()
}

// processMessage 处理单条消息
func (e *Engine) processMessage(msg *weixin.IncomingMessage) {
	start := time.Now()

	agentCtx, cancel := context.WithTimeout(e.ctx, agentTimeout)
	defer cancel()

	finalContent, err := e.agent.Send(agentCtx, msg.Content)
	elapsed := time.Since(start)
	if err != nil {
		if errors.Is(e.ctx.Err(), context.Canceled) {
			return
		}
		if errors.Is(agentCtx.Err(), context.DeadlineExceeded) {
			log.Printf("⏰ Claude Code 响应超时 (%v)", elapsed)
			_ = e.wechat.SendMessage(e.ctx, msg.FromUserID, "⚠️ Claude Code 响应超时，请重试")
			return
		}

		errMsg := fmt.Sprintf("⚠️ Claude Code 错误: %s", err)
		log.Printf("❌ %s", errMsg)
		_ = e.wechat.SendMessage(e.ctx, msg.FromUserID, errMsg)
		return
	}

	finalContent = strings.TrimSpace(finalContent)
	if strings.EqualFold(finalContent, "NO_REPLY") || finalContent == "" {
		log.Printf("🔇 Claude Code 返回空，不发送 (%v)", elapsed)
		return
	}

	log.Printf("📤 发送回复到 %s (%d 字符, %v)", shortID(msg.FromUserID), utf8.RuneCountInString(finalContent), elapsed)
	e.sendToUser(msg.FromUserID, finalContent)
}

// sendToUser 发送消息给用户（自动分块）
func (e *Engine) sendToUser(userID, content string) {
	if userID == "" || content == "" {
		return
	}
	chunks := splitChunks(content, maxSendLen)
	for i, chunk := range chunks {
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		if err := e.wechat.SendMessage(e.ctx, userID, chunk); err != nil {
			log.Printf("❌ 发送消息失败: %v", err)
			return
		}
	}
}

// --- 工具函数 ---

func shortID(id string) string {
	if len(id) > 20 {
		return id[:20] + "…"
	}
	return id
}

func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func splitChunks(s string, max int) []string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return []string{s}
	}
	var out []string
	runes := []rune(s)
	for len(runes) > 0 {
		n := max
		if len(runes) < n {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}
