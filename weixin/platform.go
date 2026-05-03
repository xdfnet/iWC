package weixin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	typingTicketTTL      = 10 * time.Minute
	typingRepeatInterval = 5 * time.Second
	typingStopTimeout    = 3 * time.Second
)

// MessageHandler 处理收到的微信消息
type MessageHandler func(msg *IncomingMessage)

// Platform 微信 ilink 平台，负责长轮询消息并转发给引擎
type Platform struct {
	client     *Client
	token      string
	baseURL    string
	allowFrom  string
	longPollMS int

	mu      sync.RWMutex
	handler MessageHandler
	cancel  context.CancelFunc

	syncBuf   string
	syncBufMu sync.Mutex
	syncPath  string
	dedup     map[string]time.Time
	dedupMu   sync.Mutex

	tokens    map[string]string
	tokensMu  sync.RWMutex
	tokenPath string

	typingMu      sync.RWMutex
	typingTickets map[string]typingTicket
}

type typingTicket struct {
	value     string
	fetchedAt time.Time
}

func NewPlatform(token, baseURL, allowFrom string, longPollMS int, dataDir ...string) *Platform {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	p := &Platform{
		client:        NewClient(baseURL, token),
		token:         token,
		baseURL:       baseURL,
		allowFrom:     allowFrom,
		longPollMS:    longPollMS,
		dedup:         make(map[string]time.Time),
		tokens:        make(map[string]string),
		typingTickets: make(map[string]typingTicket),
	}

	if len(dataDir) > 0 && strings.TrimSpace(dataDir[0]) != "" {
		p.initState(filepath.Join(strings.TrimSpace(dataDir[0]), "wechat"))
	}

	return p
}

func (p *Platform) initState(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("⚠️ 创建微信状态目录失败: %v", err)
		return
	}
	p.syncPath = filepath.Join(dir, "get_updates.buf")
	p.tokenPath = filepath.Join(dir, "context_tokens.json")
	p.loadSyncBuf()
	p.loadTokens()
}

func (p *Platform) loadSyncBuf() {
	if p.syncPath == "" {
		return
	}
	b, err := os.ReadFile(p.syncPath)
	if err != nil {
		return
	}
	p.syncBufMu.Lock()
	p.syncBuf = string(b)
	p.syncBufMu.Unlock()
}

func (p *Platform) persistSyncBuf(buf string) {
	p.syncBufMu.Lock()
	p.syncBuf = buf
	path := p.syncPath
	p.syncBufMu.Unlock()

	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(buf), 0600); err != nil {
		log.Printf("⚠️ 保存 get_updates 游标失败: %v", err)
	}
}

func (p *Platform) loadTokens() {
	if p.tokenPath == "" {
		return
	}
	b, err := os.ReadFile(p.tokenPath)
	if err != nil {
		return
	}
	var tokens map[string]string
	if err := json.Unmarshal(b, &tokens); err != nil {
		log.Printf("⚠️ 读取 context_token 缓存失败: %v", err)
		return
	}
	if tokens == nil {
		tokens = make(map[string]string)
	}
	p.tokensMu.Lock()
	p.tokens = tokens
	p.tokensMu.Unlock()
}

func (p *Platform) persistTokens() {
	if p.tokenPath == "" {
		return
	}
	p.tokensMu.RLock()
	out, err := json.MarshalIndent(p.tokens, "", "  ")
	p.tokensMu.RUnlock()
	if err != nil {
		log.Printf("⚠️ 编码 context_token 缓存失败: %v", err)
		return
	}
	if err := os.WriteFile(p.tokenPath, out, 0600); err != nil {
		log.Printf("⚠️ 保存 context_token 缓存失败: %v", err)
	}
}

func (p *Platform) setContextToken(userID, token string) {
	if userID == "" || token == "" {
		return
	}
	p.tokensMu.Lock()
	p.tokens[userID] = token
	p.tokensMu.Unlock()
	p.persistTokens()
}

func (p *Platform) getContextToken(userID string) string {
	p.tokensMu.RLock()
	defer p.tokensMu.RUnlock()
	return p.tokens[userID]
}

// SetContextTokenForTest 测试辅助：设置 context_token（绕过消息接收流程）
func (p *Platform) SetContextTokenForTest(userID, token string) {
	p.setContextToken(userID, token)
}

func (p *Platform) getTypingTicket(ctx context.Context, userID, contextToken string) string {
	p.typingMu.RLock()
	entry, ok := p.typingTickets[userID]
	p.typingMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < typingTicketTTL {
		return entry.value
	}

	ticket, err := p.client.GetTypingTicket(ctx, userID, contextToken)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("⚠️ 获取 typing_ticket 失败: %v", err)
		}
		return ""
	}
	if ticket == "" {
		return ""
	}

	p.typingMu.Lock()
	p.typingTickets[userID] = typingTicket{value: ticket, fetchedAt: time.Now()}
	p.typingMu.Unlock()
	return ticket
}

// StartTyping 在 Claude Code 处理期间显示“正在输入中”。
func (p *Platform) StartTyping(ctx context.Context, userID string) func() {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return func() {}
	}

	typingCtx, cancel := context.WithCancel(ctx)
	var once sync.Once
	go p.runTyping(typingCtx, userID)

	return func() {
		once.Do(cancel)
	}
}

func (p *Platform) runTyping(ctx context.Context, userID string) {
	contextToken := p.getContextToken(userID)
	if contextToken == "" {
		return
	}

	ticket := p.getTypingTicket(ctx, userID, contextToken)
	if ticket == "" {
		return
	}

	// 无论 start 是否发送成功，都确保发送 stop（服务端自己也有 TTL）
	defer p.stopTyping(userID, ticket)

	if err := p.client.SendTyping(ctx, userID, ticket, typingStatusStart); err != nil {
		if ctx.Err() == nil {
			log.Printf("⚠️ 发送正在输入状态失败: %v", err)
		}
		return
	}

	ticker := time.NewTicker(typingRepeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.client.SendTyping(ctx, userID, ticket, typingStatusStart); err != nil {
				if ctx.Err() == nil {
					log.Printf("⚠️ 刷新正在输入状态失败: %v", err)
				}
				return
			}
		}
	}
}

func (p *Platform) stopTyping(userID, ticket string) {
	ctx, cancel := context.WithTimeout(context.Background(), typingStopTimeout)
	defer cancel()
	if err := p.client.SendTyping(ctx, userID, ticket, typingStatusStop); err != nil {
		log.Printf("⚠️ 停止正在输入状态失败: %v", err)
	}
}

// Start 开始消息轮询（幂等）
func (p *Platform) Start(handler MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return nil // 已启动
	}
	p.handler = handler
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.pollLoop(ctx)
	return nil
}

// Stop 停止轮询
func (p *Platform) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// SendMessage 向外发送文本消息
func (p *Platform) SendMessage(ctx context.Context, toUserID, text string) error {
	token := p.getContextToken(toUserID)
	if token == "" {
		return fmt.Errorf("没有找到用户 %s 的 context_token（需要用户先发消息）", toUserID)
	}
	return p.client.SendText(ctx, toUserID, text, token, "")
}

const maxChunkSize = 3800

// SendMessageChunked 分块发送长文本
func (p *Platform) SendMessageChunked(ctx context.Context, toUserID, text string) error {
	token := p.getContextToken(toUserID)
	if token == "" {
		return fmt.Errorf("没有找到用户 %s 的 context_token", toUserID)
	}

	chunks := splitRunes(text, maxChunkSize)
	for i, chunk := range chunks {
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		cid := fmt.Sprintf("iwc-%s", randomHex(6))
		if err := p.client.SendText(ctx, toUserID, chunk, token, cid); err != nil {
			return fmt.Errorf("发送第 %d 块失败: %w", i+1, err)
		}
	}
	return nil
}

func (p *Platform) pollLoop(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		p.syncBufMu.Lock()
		buf := p.syncBuf
		p.syncBufMu.Unlock()

		resp, err := p.client.GetUpdates(ctx, buf, p.longPollMS)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("⚠️ getUpdates 失败: %v (%.0fs后重试)", err, backoff.Seconds())
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second

		if resp.Errcode == sessionExpiredErrcode {
			log.Println("⚠️ 会话过期，暂停 1 小时")
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Hour):
			}
			continue
		}

		if resp.GetUpdatesBuf != "" {
			p.persistSyncBuf(resp.GetUpdatesBuf)
		}

		p.mu.RLock()
		h := p.handler
		p.mu.RUnlock()
		if h == nil {
			continue
		}

		for i := range resp.Msgs {
			p.handleMessage(&resp.Msgs[i], h)
		}
	}
}

func (p *Platform) handleMessage(m *weixinMessage, h MessageHandler) {
	if m.MessageType == messageTypeBot {
		return
	}
	if m.MessageType != 0 && m.MessageType != messageTypeUser {
		return
	}
	from := strings.TrimSpace(m.FromUserID)
	if from == "" {
		return
	}
	if !allowList(p.allowFrom, from) {
		log.Printf("⚠️ 用户 %s 不在 allow_from 列表中，已忽略", from)
		return
	}

	// 去重
	dk := fmt.Sprintf("%s|%d|%d", from, m.MessageID, m.CreateTimeMs)
	p.dedupMu.Lock()
	now := time.Now()
	for k, t := range p.dedup {
		if now.Sub(t) > 5*time.Minute {
			delete(p.dedup, k)
		}
	}
	if _, ok := p.dedup[dk]; ok {
		p.dedupMu.Unlock()
		return
	}
	p.dedup[dk] = now
	p.dedupMu.Unlock()

	// 保存 context_token
	if tok := strings.TrimSpace(m.ContextToken); tok != "" {
		p.setContextToken(from, tok)
	}

	// 提取文本
	body := extractText(m.ItemList)
	if strings.TrimSpace(body) == "" {
		return
	}

	msgID := fmt.Sprintf("%d", m.MessageID)
	if m.MessageID == 0 {
		msgID = randomHex(8)
	}

	h(&IncomingMessage{
		FromUserID:   from,
		Content:      body,
		ContextToken: strings.TrimSpace(m.ContextToken),
		MessageID:    msgID,
	})
}

// --- 工具函数 ---

func extractText(items []messageItem) string {
	for _, it := range items {
		if it.Type == messageItemText && it.TextItem != nil {
			return it.TextItem.Text
		}
		if it.Type == messageItemVoice && it.VoiceItem != nil && it.VoiceItem.Text != "" {
			return "[语音] " + it.VoiceItem.Text
		}
	}
	return ""
}

func allowList(allowFrom, userID string) bool {
	allowFrom = strings.TrimSpace(allowFrom)
	if allowFrom == "" || allowFrom == "*" {
		return true
	}
	for _, id := range strings.Split(allowFrom, ",") {
		if strings.EqualFold(strings.TrimSpace(id), userID) {
			return true
		}
	}
	return false
}

func splitRunes(s string, max int) []string {
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
