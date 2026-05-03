package weixin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGetTypingTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getconfig" {
			t.Fatalf("path = %s, want /ilink/bot/getconfig", r.URL.Path)
		}
		if got := r.Header.Get("AuthorizationType"); got != "ilink_bot_token" {
			t.Fatalf("AuthorizationType = %q", got)
		}
		var req getConfigReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.UserID != "user@im.wechat" || req.ContextToken != "ctx-token" {
			t.Fatalf("request = %+v", req)
		}
		_ = json.NewEncoder(w).Encode(getConfigResp{TypingTicket: "typing-ticket"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	got, err := client.GetTypingTicket(context.Background(), "user@im.wechat", "ctx-token")
	if err != nil {
		t.Fatalf("GetTypingTicket returned error: %v", err)
	}
	if got != "typing-ticket" {
		t.Fatalf("typing ticket = %q, want typing-ticket", got)
	}
}

func TestClientSendTyping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendtyping" {
			t.Fatalf("path = %s, want /ilink/bot/sendtyping", r.URL.Path)
		}
		var req sendTypingReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.IlinkUserID != "user@im.wechat" || req.TypingTicket != "typing-ticket" || req.Status != typingStatusStart {
			t.Fatalf("request = %+v", req)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResp{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	if err := client.SendTyping(context.Background(), "user@im.wechat", "typing-ticket", typingStatusStart); err != nil {
		t.Fatalf("SendTyping returned error: %v", err)
	}
}

func TestGetUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(getUpdatesResp{
			Msgs: []weixinMessage{
				{MessageID: 1, FromUserID: "user1", MessageType: messageTypeUser,
					ItemList: []messageItem{{Type: messageItemText, TextItem: &textItem{Text: "hi"}}},
					ContextToken: "ctx-1"},
			},
			GetUpdatesBuf: "buf-1",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	resp, err := client.GetUpdates(context.Background(), "", 35000)
	if err != nil {
		t.Fatalf("GetUpdates error: %v", err)
	}
	if len(resp.Msgs) != 1 {
		t.Fatalf("got %d msgs, want 1", len(resp.Msgs))
	}
	if resp.Msgs[0].ContextToken != "ctx-1" {
		t.Errorf("ContextToken = %q", resp.Msgs[0].ContextToken)
	}
	if resp.GetUpdatesBuf != "buf-1" {
		t.Errorf("GetUpdatesBuf = %q", resp.GetUpdatesBuf)
	}
}

func TestGetUpdatesContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // slow enough to ensure context fires first
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetUpdates(ctx, "buf", 35000)
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
}

func TestGetUpdatesNetworkError(t *testing.T) {
	// Closed server = connection refused
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close() // close immediately

	client := NewClient(server.URL, "token")
	_, err := client.GetUpdates(context.Background(), "buf", 35000)
	if err == nil {
		t.Fatal("expected network error for closed server")
	}
}

func TestGetUpdatesTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // slow enough to exceed request timeout
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	// Use very short timeout via context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetUpdates(ctx, "buf", 35000)
	if err == nil {
		t.Fatal("expected error on timeout")
	}
}

func TestSendText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var req sendMessageReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Msg.ToUserID != "user1" {
			t.Errorf("ToUserID = %q", req.Msg.ToUserID)
		}
		if req.Msg.ContextToken != "ctx-token" {
			t.Errorf("ContextToken = %q", req.Msg.ContextToken)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResp{Ret: 0})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	err := client.SendText(context.Background(), "user1", "hello", "ctx-token", "client-123")
	if err != nil {
		t.Fatalf("SendText error: %v", err)
	}
}

func TestSendTextNoContextToken(t *testing.T) {
	client := NewClient("http://x.com", "token")
	err := client.SendText(context.Background(), "user1", "hello", "", "")
	if err == nil {
		t.Fatal("expected error for empty context_token")
	}
}

func TestSendTextAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(sendMessageResp{Ret: -1, Errcode: 100, Errmsg: "bad request"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	err := client.SendText(context.Background(), "user1", "hello", "ctx-token", "")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestSendTypingAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(sendMessageResp{Ret: -1, Errcode: 200, Errmsg: "bad"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	err := client.SendTyping(context.Background(), "user1", "ticket", typingStatusStart)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestGetTypingTicketAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(getConfigResp{Ret: -1, Errcode: -1, Errmsg: "error"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	_, err := client.GetTypingTicket(context.Background(), "user1", "ctx-token")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestGetTypingTicketEmptyUserID(t *testing.T) {
	client := NewClient("http://x.com", "token")
	_, err := client.GetTypingTicket(context.Background(), "", "ctx-token")
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

func TestGetTypingTicketEmptyContextToken(t *testing.T) {
	client := NewClient("http://x.com", "token")
	_, err := client.GetTypingTicket(context.Background(), "user1", "")
	if err == nil {
		t.Fatal("expected error for empty context_token")
	}
}

func TestSendTypingEmptyUserID(t *testing.T) {
	client := NewClient("http://x.com", "token")
	err := client.SendTyping(context.Background(), "", "ticket", typingStatusStart)
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

func TestSendTypingEmptyTicket(t *testing.T) {
	client := NewClient("http://x.com", "token")
	err := client.SendTyping(context.Background(), "user1", "", typingStatusStart)
	if err == nil {
		t.Fatal("expected error for empty typing_ticket")
	}
}

func TestDoHTTPNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	_, err := client.GetUpdates(context.Background(), "", 35000)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestDoHTTPBodyTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write more than 64MB
		big := make([]byte, maxResponseBody+1024)
		w.Write(big)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	_, err := client.GetUpdates(context.Background(), "", 35000)
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
}

func TestNewClientDefaults(t *testing.T) {
	client := NewClient("", "")
	if client.baseURL != defaultBaseURL+"/" {
		t.Errorf("baseURL = %q", client.baseURL)
	}
	client2 := NewClient("  ", " token ")
	if client2.token != "token" {
		t.Errorf("token = %q", client2.token)
	}
}

func TestRandomHex(t *testing.T) {
	s1 := randomHex(6)
	s2 := randomHex(6)
	if len(s1) != 12 { // hex encoding doubles length
		t.Errorf("len = %d, want 12", len(s1))
	}
	if s1 == s2 {
		t.Error("random hex should differ between calls")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello…"},
		{"hi", 2, "hi"},
	}
	for _, tt := range tests {
		got := truncate([]byte(tt.in), tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"context deadline", context.DeadlineExceeded, true}, // implements net.Error.Timeout
		{"net timeout", &net.OpError{Op: "dial", Err: &errTimeout{}}, true},
	}
	for _, tt := range tests {
		got := isTimeout(tt.err)
		if got != tt.want {
			t.Errorf("isTimeout(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

type errTimeout struct{}

func (e *errTimeout) Error() string   { return "timeout" }
func (e *errTimeout) Timeout() bool   { return true }
func (e *errTimeout) Temporary() bool { return true }

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"net error", &net.OpError{Op: "dial", Err: &errTimeout{}}, true},
		{"connection refused", errors.New("connection refused"), true},
		{"no such host", errors.New("no such host"), true},
		{"generic error", errors.New("something"), false},
	}
	for _, tt := range tests {
		got := isNetworkError(tt.err)
		if got != tt.want {
			t.Errorf("isNetworkError(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestQRStatusMessage(t *testing.T) {
	cases := map[string]string{
		"":          "等待扫码中...",
		"wait":      "等待扫码中...",
		"scaned":    "已扫码，等待手机确认...",
		"expired":   "二维码已过期，正在刷新...",
		"confirmed": "已确认登录，正在完成配置...",
		"other":     "处理中...",
	}

	for input, want := range cases {
		got := qrStatusMessage(input)
		if got != want {
			t.Fatalf("status %q => %q, want %q", input, got, want)
		}
	}
}
