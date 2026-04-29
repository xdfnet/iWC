package weixin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
