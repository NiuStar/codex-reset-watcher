package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWatcherBaselineThenNotifyOnce(t *testing.T) {
	posts := []string{"10"}
	sends := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/users/by/username/thsottiaux":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"id": "42", "username": "thsottiaux"}})
		case "/2/users/42/tweets":
			if r.URL.Query().Get("post.fields") != "text,created_at,note_post" || r.URL.Query().Get("expansions") != "author_id,referenced_posts" || r.URL.Query().Get("exclude") != "retweets" {
				t.Errorf("bad X query: %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("since_id") == "" {
				json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"id": "10", "author_id": "42", "text": "old reset"}}, "meta": map[string]int{"result_count": 1}})
				return
			}
			if len(posts) == 1 || r.URL.Query().Get("since_id") == "11" {
				json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"result_count": 0}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"id": "11", "author_id": "42", "text": "the reset has been processed"}}, "meta": map[string]int{"result_count": 1}})
		case "/v1/chat/completions":
			w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"category\":\"COMPLETED\",\"evidence\":\"the reset has been processed\",\"reason\":\"done\"}"}}]}`))
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Write([]byte(`{"code":0,"tenant_access_token":"test-token"}`))
		case "/open-apis/im/v1/messages":
			sends++
			if r.URL.Query().Get("receive_id_type") != "chat_id" || r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("Feishu request metadata invalid")
			}
			var req struct {
				ReceiveID string `json:"receive_id"`
				MsgType   string `json:"msg_type"`
				Content   string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			if req.ReceiveID != "oc_test" || req.MsgType != "text" || !strings.Contains(req.Content, "/status/11") || !strings.Contains(req.Content, "the reset has been processed") {
				t.Errorf("Feishu message invalid: %+v", req)
			}
			w.Write([]byte(`{"code":0,"data":{"message_id":"msg-1"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	cfg := watchConfig{XBase: srv.URL, XToken: "x", AIBase: srv.URL, AIKey: "a", Model: "model", FeishuBase: srv.URL, AppID: "app", AppSecret: "secret", ChatID: "oc_test", StatePath: filepath.Join(dir, "state.json")}
	for _, want := range []int{0, 1, 1} {
		if want == 1 {
			posts = append(posts, "11")
		}
		if err := runWatchTick(context.Background(), srv.Client(), cfg); err != nil {
			t.Fatal(err)
		}
		if sends != want {
			t.Fatalf("sends=%d want=%d", sends, want)
		}
	}
	state, err := os.ReadFile(cfg.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), `"since_id": "11"`) {
		t.Fatal(string(state))
	}
}

func TestWatcherSendFailureDoesNotAdvance(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/users/by/username/thsottiaux":
			w.Write([]byte(`{"data":{"id":"42","username":"thsottiaux"}}`))
		case "/2/users/42/tweets":
			w.Write([]byte(`{"data":[{"id":"11","author_id":"42","text":"reset processed"}],"meta":{"result_count":1}}`))
		case "/v1/chat/completions":
			w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"category\":\"COMPLETED\",\"evidence\":\"reset processed\",\"reason\":\"done\"}"}}]}`))
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Write([]byte(`{"code":0,"tenant_access_token":"token"}`))
		case "/open-apis/im/v1/messages":
			calls++
			w.Write([]byte(`{"code":999,"msg":"failed"}`))
		default:
			t.Errorf("bad path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	cfg := watchConfig{XBase: srv.URL, XToken: "x", AIBase: srv.URL, AIKey: "a", Model: "m", FeishuBase: srv.URL, AppID: "i", AppSecret: "s", ChatID: "oc_test", StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := saveWatchState(cfg.StatePath, watchState{UserID: "42", SinceID: "10"}); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), srv.Client(), cfg); err == nil {
		t.Fatal("expected Feishu failure")
	}
	state, err := loadWatchState(cfg.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.SinceID != "10" || calls != 1 {
		t.Fatalf("state=%+v calls=%d", state, calls)
	}
}

func TestWatcherPaginationFailureDoesNotAdvance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/users/by/username/thsottiaux":
			w.Write([]byte(`{"data":{"id":"42","username":"thsottiaux"}}`))
		case "/2/users/42/tweets":
			if r.URL.Query().Get("pagination_token") != "" {
				w.WriteHeader(503)
				return
			}
			w.Write([]byte(`{"data":[{"id":"11","author_id":"42","text":"reset"}],"meta":{"next_token":"next","result_count":1}}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	cfg := watchConfig{XBase: srv.URL, XToken: "x", StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := saveWatchState(cfg.StatePath, watchState{UserID: "42", SinceID: "10"}); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), srv.Client(), cfg); err == nil {
		t.Fatal("expected pagination failure")
	}
	state, _ := loadWatchState(cfg.StatePath)
	if state.SinceID != "10" {
		t.Fatalf("advanced: %+v", state)
	}
}
