package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/admin/iCode/iWC/claude"
	"github.com/admin/iCode/iWC/weixin"
)

const maxSendLen = 3800
const agentTimeout = 120 * time.Second

// Engine 桥接 WeChat 消息和 Claude Code
type Engine struct {
	wechat *weixin.Platform
	agent  *claude.Agent
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	started   bool
	startedMu sync.Mutex

	sessions   map[string]string // userID -> sessionID
	sessionsMu sync.RWMutex
	sessPath   string
}

// New 创建引擎
func New(wechat *weixin.Platform, agent *claude.Agent) *Engine {
	return &Engine{
		wechat:   wechat,
		agent:    agent,
		sessions: make(map[string]string),
	}
}

// Start 启动引擎（幂等，重复调用安全）
func (e *Engine) Start(ctx context.Context) error {
	e.startedMu.Lock()
	if e.started {
		e.startedMu.Unlock()
		return nil
	}
	e.started = true
	e.startedMu.Unlock()

	e.ctx, e.cancel = context.WithCancel(ctx)

	// 加载会话数据
	e.loadSessions()

	// 启动微信消息监听
	if err := e.wechat.Start(e.handleWeChatMessage); err != nil {
		e.startedMu.Lock()
		e.started = false
		e.startedMu.Unlock()
		return fmt.Errorf("启动微信平台失败: %w", err)
	}

	log.Println("✅ iWC 引擎已启动")
	log.Println("   Claude Code 就绪，等待微信消息...")

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.startedMu.Lock()
	defer e.startedMu.Unlock()
	if !e.started {
		return
	}
	e.started = false
	if e.cancel != nil {
		e.cancel()
	}
	e.wechat.Stop()
	e.wg.Wait()
	log.Println("iWC 引擎已停止")
}

// loadSessions 从磁盘加载会话映射
func (e *Engine) loadSessions() {
	if e.sessPath == "" {
		return
	}
	b, err := os.ReadFile(e.sessPath)
	if err != nil {
		return
	}
	var sessions map[string]string
	if err := json.Unmarshal(b, &sessions); err != nil {
		return
	}
	if sessions == nil {
		sessions = make(map[string]string)
	}
	e.sessionsMu.Lock()
	e.sessions = sessions
	e.sessionsMu.Unlock()
}

// persistSessions 保存会话映射到磁盘
func (e *Engine) persistSessions() {
	if e.sessPath == "" {
		return
	}
	e.sessionsMu.RLock()
	out, err := json.MarshalIndent(e.sessions, "", "  ")
	e.sessionsMu.RUnlock()
	if err != nil {
		log.Printf("⚠️ 编码 sessions 失败: %v", err)
		return
	}
	if err := os.WriteFile(e.sessPath, out, 0600); err != nil {
		log.Printf("⚠️ 保存 sessions 失败: %v", err)
	}
}

// SetSessionsPath 设置会话文件路径
func (e *Engine) SetSessionsPath(path string) {
	e.sessPath = path
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

	// 获取用户的 session
	e.sessionsMu.RLock()
	sessionID := e.sessions[msg.FromUserID]
	e.sessionsMu.RUnlock()

	agentCtx, cancel := context.WithTimeout(e.ctx, agentTimeout)
	stopTyping := e.wechat.StartTyping(agentCtx, msg.FromUserID)
	defer func() {
		stopTyping()
		cancel()
	}()

	// 发送消息（使用 session 恢复对话）
	finalContent, newSessionID, err := e.agent.SendWithSession(agentCtx, msg.Content, sessionID)
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

	// 保存/更新 sessionID（处理新建、过期重试等场景）
	if newSessionID != "" && newSessionID != sessionID {
		e.sessionsMu.Lock()
		e.sessions[msg.FromUserID] = newSessionID
		e.sessionsMu.Unlock()
		e.persistSessions()
		log.Printf("💾 会话已保存 for %s", shortID(msg.FromUserID))
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