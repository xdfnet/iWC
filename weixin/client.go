package weixin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL     = "https://ilinkai.weixin.qq.com"
	defaultAPITimeout  = 15 * time.Second
	defaultLongPollDur = 35 * time.Second
	maxResponseBody    = 64 << 20 // 64MB
	channelVersion     = "icc-weixin/1.0"
)

// Client 管理 ilink HTTP API 调用
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/") + "/"
	return &Client{
		baseURL:    baseURL,
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: defaultAPITimeout},
	}
}

// GetUpdates 长轮询获取新消息
func (c *Client) GetUpdates(ctx context.Context, buf string, timeoutMs int) (*getUpdatesResp, error) {
	timeout := defaultLongPollDur
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}
	req := getUpdatesReq{
		GetUpdatesBuf: buf,
		BaseInfo:      baseInfo{ChannelVersion: channelVersion},
	}
	payload, _ := json.Marshal(req)
	raw, err := c.post(ctx, "ilink/bot/getupdates", payload, timeout)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return &getUpdatesResp{Ret: 0, Msgs: nil, GetUpdatesBuf: buf}, nil
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return &getUpdatesResp{Ret: 0, Msgs: nil, GetUpdatesBuf: buf}, nil
		}
		return nil, fmt.Errorf("getUpdates: %w", err)
	}
	var out getUpdatesResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("getUpdates json: %w", err)
	}
	return &out, nil
}

// SendText 发送文本消息给指定用户
func (c *Client) SendText(ctx context.Context, to, text, contextToken, clientID string) error {
	if strings.TrimSpace(contextToken) == "" {
		return fmt.Errorf("context_token is required")
	}
	if clientID == "" {
		clientID = "icc-" + randomHex(6)
	}
	items := []messageItem{
		{Type: messageItemText, TextItem: &textItem{Text: text}},
	}
	msg := sendMessageReq{
		Msg: weixinOutboundMsg{
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  messageTypeBot,
			MessageState: messageStateFinish,
			ItemList:     items,
			ContextToken: contextToken,
		},
		BaseInfo: baseInfo{ChannelVersion: channelVersion},
	}
	payload, _ := json.Marshal(msg)
	raw, err := c.post(ctx, "ilink/bot/sendmessage", payload, 0)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var resp sendMessageResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("sendMessage json: %w", err)
	}
	if resp.Ret != 0 {
		return fmt.Errorf("sendMessage: ret=%d errcode=%d errmsg=%s", resp.Ret, resp.Errcode, resp.Errmsg)
	}
	return nil
}

// GetTypingTicket 获取发送“正在输入中”所需的 typing_ticket
func (c *Client) GetTypingTicket(ctx context.Context, userID, contextToken string) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(contextToken) == "" {
		return "", fmt.Errorf("context_token is required")
	}
	req := getConfigReq{
		UserID:       userID,
		ContextToken: contextToken,
		BaseInfo:     baseInfo{ChannelVersion: channelVersion},
	}
	payload, _ := json.Marshal(req)
	raw, err := c.post(ctx, "ilink/bot/getconfig", payload, 0)
	if err != nil {
		return "", err
	}
	var resp getConfigResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("getConfig json: %w", err)
	}
	if resp.Ret != 0 || resp.Errcode != 0 {
		return "", fmt.Errorf("getConfig: ret=%d errcode=%d errmsg=%s", resp.Ret, resp.Errcode, resp.Errmsg)
	}
	return strings.TrimSpace(resp.TypingTicket), nil
}

// SendTyping 发送正在输入状态。status: 1=start, 2=stop。
func (c *Client) SendTyping(ctx context.Context, userID, typingTicket string, status int) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(typingTicket) == "" {
		return fmt.Errorf("typing_ticket is required")
	}
	req := sendTypingReq{
		IlinkUserID:  userID,
		TypingTicket: typingTicket,
		Status:       status,
		BaseInfo:     baseInfo{ChannelVersion: channelVersion},
	}
	payload, _ := json.Marshal(req)
	raw, err := c.post(ctx, "ilink/bot/sendtyping", payload, 0)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var resp sendMessageResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("sendTyping json: %w", err)
	}
	if resp.Ret != 0 || resp.Errcode != 0 {
		return fmt.Errorf("sendTyping: ret=%d errcode=%d errmsg=%s", resp.Ret, resp.Errcode, resp.Errmsg)
	}
	return nil
}

// --- 扫码登录 API ---

type BotQRResponse struct {
	QRCode           string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

type QRStatusResponse struct {
	Status      string `json:"status"`
	BotToken    string `json:"bot_token"`
	IlinkBotID  string `json:"ilink_bot_id"`
	BaseURL     string `json:"baseurl"`
	IlinkUserID string `json:"ilink_user_id"`
}

func (c *Client) GetBotQRCode(ctx context.Context, botType string) (*BotQRResponse, error) {
	if botType == "" {
		botType = "3"
	}
	u := c.baseURL + "ilink/bot/get_bot_qrcode?bot_type=" + botType
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	raw, err := c.doHTTP(req, 0)
	if err != nil {
		return nil, fmt.Errorf("get_bot_qrcode: %w", err)
	}
	var out BotQRResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("get_bot_qrcode json: %w", err)
	}
	return &out, nil
}

func (c *Client) PollQRStatus(ctx context.Context, qrKey string) (*QRStatusResponse, error) {
	u := c.baseURL + "ilink/bot/get_qrcode_status?qrcode=" + qrKey
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("iLink-App-ClientVersion", "1")
	raw, err := c.doHTTP(req, defaultLongPollDur+5*time.Second)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return &QRStatusResponse{Status: "wait"}, nil
		}
		return nil, err
	}
	var out QRStatusResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("get_qrcode_status json: %w", err)
	}
	return &out, nil
}

func (c *Client) VerifyToken(ctx context.Context) error {
	body := []byte(`{"get_updates_buf":"","base_info":{"channel_version":"icc-verify/1.0"}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"ilink/bot/getupdates", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-WECHAT-UIN", randomWechatUIN())
	_, err = c.doHTTP(req, 15*time.Second)
	return err
}

// --- 内部 HTTP 方法 ---

func (c *Client) post(ctx context.Context, endpoint string, body []byte, timeout time.Duration) ([]byte, error) {
	url := c.baseURL + strings.TrimPrefix(endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-WECHAT-UIN", randomWechatUIN())
	return c.doHTTP(req, timeout)
}

func (c *Client) doHTTP(req *http.Request, timeout time.Duration) ([]byte, error) {
	client := c.httpClient
	if timeout > 0 {
		client = &http.Client{Timeout: timeout + 5*time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxResponseBody {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxResponseBody)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(raw, 256))
	}
	return raw, nil
}

// --- 工具函数 ---

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func randomWechatUIN() string {
	var b [4]byte
	rand.Read(b[:])
	u := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", u)))
}

func truncate(b []byte, max int) string {
	s := string(b)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network is unreachable")
}

// QRLogin 执行完整的扫码登录流程（命令行用）
func QRLogin(ctx context.Context, apiBaseURL, botType string, timeout time.Duration) (token, baseURL, ilinkBotID, ilinkUserID string, err error) {
	c := NewClient(apiBaseURL, "")
	if timeout < time.Second {
		timeout = 480 * time.Second
	}
	deadline := time.Now().Add(timeout)

	qrResp, err := c.GetBotQRCode(ctx, botType)
	if err != nil {
		return "", "", "", "", fmt.Errorf("获取二维码失败: %w", err)
	}
	qrURL := qrResp.QRCodeImgContent
	log.Printf("请使用微信扫描二维码:\n%s\n", qrURL)

	qrKey := qrResp.QRCode
	refreshCount := 0
	const maxRefresh = 3

	for time.Now().Before(deadline) {
		status, err := c.PollQRStatus(ctx, qrKey)
		if err != nil {
			// 网络瞬时错误重试一次
			if isTimeout(err) || isNetworkError(err) {
				log.Printf("⚠️ 轮询二维码状态失败 (%v)，重试...", err)
				time.Sleep(2 * time.Second)
				status, err = c.PollQRStatus(ctx, qrKey)
			}
			if err != nil {
				return "", "", "", "", err
			}
		}
		switch status.Status {
		case "wait", "":
			time.Sleep(time.Second)
		case "scaned":
			log.Println("已扫码，请在手机上确认登录…")
			time.Sleep(time.Second)
		case "expired":
			refreshCount++
			if refreshCount > maxRefresh {
				return "", "", "", "", fmt.Errorf("二维码多次过期，请重试")
			}
			log.Printf("二维码已过期，正在刷新 (%d/%d)…\n", refreshCount, maxRefresh)
			newQR, err := c.GetBotQRCode(ctx, botType)
			if err != nil {
				return "", "", "", "", fmt.Errorf("刷新二维码: %w", err)
			}
			qrKey = newQR.QRCode
			log.Printf("请扫描新二维码:\n%s\n", newQR.QRCodeImgContent)
			time.Sleep(time.Second)
		case "confirmed":
			if status.IlinkBotID == "" || status.BotToken == "" {
				return "", "", "", "", fmt.Errorf("登录确认但缺少 bot_token 或 ilink_bot_id")
			}
			log.Println("✅ 微信登录成功")
			baseURL := apiBaseURL
			if status.BaseURL != "" {
				baseURL = status.BaseURL
			}
			return status.BotToken, baseURL, status.IlinkBotID, status.IlinkUserID, nil
		default:
			time.Sleep(time.Second)
		}
	}
	return "", "", "", "", fmt.Errorf("等待扫码超时")
}
