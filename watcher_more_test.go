package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The API returns reverse chronological pages, but events must be delivered oldest first.
func TestWatcherMultiplePagesOldestFirst(t *testing.T) {
	var delivered []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/users/by/username/thsottiaux":
			w.Write([]byte(`{"data":{"id":"42","username":"thsottiaux"}}`))
		case "/2/users/42/tweets":
			if r.URL.Query().Get("pagination_token") == "" {
				w.Write([]byte(`{"data":[{"id":"13","author_id":"42","text":"reset 13"}],"meta":{"next_token":"more","result_count":1}}`))
			} else {
				w.Write([]byte(`{"data":[{"id":"12","author_id":"42","text":"reset 12"},{"id":"11","author_id":"42","text":"reset 11"}],"meta":{"result_count":2}}`))
			}
		case "/v1/chat/completions":
			var req struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			var p Post
			json.Unmarshal([]byte(req.Messages[1].Content), &p)
			b, _ := json.Marshal(Decision{Category: "COMPLETED", Evidence: p.Text, Reason: "done"})
			resp, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(b)}}}})
			w.Write(resp)
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Write([]byte(`{"code":0,"tenant_access_token":"token"}`))
		case "/open-apis/im/v1/messages":
			var req struct {
				Content string `json:"content"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			var content struct {
				Text string `json:"text"`
			}
			json.Unmarshal([]byte(req.Content), &content)
			for _, id := range []string{"11", "12", "13"} {
				if len(content.Text) > 0 && containsPostURL(content.Text, id) {
					delivered = append(delivered, id)
					break
				}
			}
			w.Write([]byte(`{"code":0,"data":{"message_id":"msg"}}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	cfg := watchConfig{XBase: srv.URL, XToken: "x", AIBase: srv.URL, AIKey: "a", Model: "m", FeishuBase: srv.URL, AppID: "i", AppSecret: "s", ChatID: "oc_test", StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := saveWatchState(cfg.StatePath, watchState{UserID: "42", SinceID: "10"}); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), srv.Client(), cfg); err != nil {
		t.Fatal(err)
	}
	if len(delivered) != 3 || delivered[0] != "11" || delivered[1] != "12" || delivered[2] != "13" {
		t.Fatalf("order: %v", delivered)
	}
	state, _ := loadWatchState(cfg.StatePath)
	if state.SinceID != "13" {
		t.Fatalf("state: %+v", state)
	}
}
func containsPostURL(text, id string) bool { return len(text) > 0 && contains(text, "/status/"+id) }
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestWatcherConfirmedPendingRecovery(t *testing.T) {
	sends := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/users/by/username/thsottiaux":
			w.Write([]byte(`{"data":{"id":"42","username":"thsottiaux"}}`))
		case "/2/users/42/tweets":
			w.Write([]byte(`{"meta":{"result_count":0}}`))
		case "/open-apis/im/v1/messages":
			sends++
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	cfg := watchConfig{XBase: srv.URL, XToken: "x", StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := saveWatchState(cfg.StatePath, watchState{UserID: "42", SinceID: "10", PendingID: "11", PendingCategory: "COMPLETED", MessageID: "msg-1"}); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), srv.Client(), cfg); err != nil {
		t.Fatal(err)
	}
	st, _ := loadWatchState(cfg.StatePath)
	if st.SinceID != "11" || st.PendingID != "" || sends != 0 {
		t.Fatalf("state=%+v sends=%d", st, sends)
	}
}
func TestWatcherAmbiguousPendingStopsBeforeNetwork(t *testing.T) {
	cfg := watchConfig{StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := saveWatchState(cfg.StatePath, watchState{UserID: "42", SinceID: "10", PendingID: "11", PendingCategory: "COMPLETED"}); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), &http.Client{}, cfg); err == nil {
		t.Fatal("ambiguous pending was retried")
	}
	if _, err := os.Stat(cfg.StatePath); err != nil {
		t.Fatal(err)
	}
}
