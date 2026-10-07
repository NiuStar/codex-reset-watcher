package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBankedMessageDoesNotClaimAlreadyDelivered(t *testing.T) {
	var message string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Write([]byte(`{"code":0,"tenant_access_token":"fixture"}`))
		case "/open-apis/im/v1/messages":
			var req struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			var content struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(req.Content), &content); err != nil {
				t.Error(err)
			}
			message = content.Text
			w.Write([]byte(`{"code":0,"data":{"message_id":"fixture-message"}}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	post := xPost{ID: "2107913674593644711", Text: "Loading a banked reset in everyone's paid accounts. See you again tomorrow!"}
	d := Decision{Category: "BANKED", Scope: "paid accounts", Audience: "everyone", Evidence: "Loading a banked reset in everyone's paid accounts."}
	cfg := watchConfig{FeishuBase: srv.URL, AppID: "fixture", AppSecret: "fixture", ChatID: "oc_fixture"}
	if _, err := sendFeishu(context.Background(), srv.Client(), cfg, post, d); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message, "已发放") || strings.Contains(message, "已到账") {
		t.Fatalf("premature completion claim: %s", message)
	}
	if !strings.Contains(message, "正在发放") || !strings.Contains(message, "不代表") {
		t.Fatalf("missing in-progress and caveat: %s", message)
	}
	if !strings.Contains(message, "/status/2107913674593644711") {
		t.Fatal("missing source URL")
	}
}
