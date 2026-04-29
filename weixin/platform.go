package weixin

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// MessageHandler 处理收到的微信消息
type MessageHandler func(msg *IncomingMessage)

// Platform 微信 ilink 平台，负责长轮询消息并转发给引擎
type Platform struct {
	client      *Client
	token       string
	baseURL     string
	allowFrom   string
	longPollMS  int

	mu       sync.RWMutex
	handler  MessageHandler
	cancel   context.CancelFunc

	syncBuf   string
	syncBufMu sync.Mutex
	dedup     map[string]time.Time
	dedupMu   sync.Mutex

	tokens   map[string]string
	tokensMu sync.RWMutex
}

func NewPlatform(token, baseURL, allowFrom string, longPollMS int) *Platform {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Platform{
		client:     NewClient(baseURL, token),
		token:      token,
		baseURL:    baseURL,
		allowFrom:  allowFrom,
		longPollMS: longPollMS,
		dedup:      make(map[string]time.Time),
		tokens:     make(map[string]string),
	}
}

// Start 开始消息轮询
func (p *Platform) Start(handler MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
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
	p.tokensMu.RLock()
	token := p.tokens[toUserID]
	p.tokensMu.RUnlock()
	if token == "" {
		return fmt.Errorf("没有找到用户 %s 的 context_token（需要用户先发消息）", toUserID)
	}
	return p.client.SendText(ctx, toUserID, text, token, "")
}

const maxChunkSize = 3800

// SendMessageChunked 分块发送长文本
func (p *Platform) SendMessageChunked(ctx context.Context, toUserID, text string) error {
	p.tokensMu.RLock()
	token := p.tokens[toUserID]
	p.tokensMu.RUnlock()
	if token == "" {
		return fmt.Errorf("没有找到用户 %s 的 context_token", toUserID)
	}

	chunks := splitRunes(text, maxChunkSize)
	for i, chunk := range chunks {
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		cid := fmt.Sprintf("icc-%s", randomHex(6))
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

		p.syncBufMu.Lock()
		if resp.GetUpdatesBuf != "" {
			p.syncBuf = resp.GetUpdatesBuf
		}
		p.syncBufMu.Unlock()

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
		p.tokensMu.Lock()
		p.tokens[from] = tok
		p.tokensMu.Unlock()
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
