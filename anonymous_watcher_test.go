package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnonymousWatcherBaselineThenNotify(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC1123)
	newest := "2107676072871600471"
	older := "2107676072871600470"
	feed := func(extra bool) string {
		items := ""
		if extra {
			items += fmt.Sprintf(`<item><description>the reset has been processed</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">@thsottiaux</dc:creator><link>https://mirror.example/thsottiaux/status/%s</link><pubDate>%s</pubDate></item>`, newest, now)
		}
		items += fmt.Sprintf(`<item><description>old news</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">@thsottiaux</dc:creator><link>https://mirror.example/thsottiaux/status/%s</link><pubDate>%s</pubDate></item>`, older, now)
		return `<rss><channel><title>Tibo (@thsottiaux)</title>` + items + `</channel></rss>`
	}
	withNew := false
	sends := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/thsottiaux/rss":
			w.Write([]byte(strings.ReplaceAll(feed(withNew), "mirror.example", r.Host)))
		case "/thsottiaux/status/2107676072871600471":
			w.Write([]byte(`{"code":200,"tweet":{"id":"2107676072871600471","text":"the reset has been processed","author":{"screen_name":"thsottiaux","id":"1953337039510003712"}}}`))
		case "/thsottiaux/status/2107676072871600470":
			w.Write([]byte(`{"code":200,"tweet":{"id":"2107676072871600470","text":"old news","author":{"screen_name":"thsottiaux","id":"1953337039510003712"}}}`))
		case "/v1/chat/completions":
			w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"category\":\"COMPLETED\",\"evidence\":\"the reset has been processed\",\"reason\":\"done\"}"}}]}`))
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Write([]byte(`{"code":0,"tenant_access_token":"token"}`))
		case "/open-apis/im/v1/messages":
			sends++
			w.Write([]byte(`{"code":0,"data":{"message_id":"msg"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := watchConfig{FeedURL: srv.URL + "/thsottiaux/rss", VerifyBase: srv.URL, AIBase: srv.URL, AIKey: "key", Model: "model", FeishuBase: srv.URL, AppID: "app", AppSecret: "secret", ChatID: "oc_test", StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := runWatchTick(context.Background(), srv.Client(), c); err != nil {
		t.Fatal(err)
	}
	withNew = true
	if err := runWatchTick(context.Background(), srv.Client(), c); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), srv.Client(), c); err != nil {
		t.Fatal(err)
	}
	state, err := loadWatchState(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if sends != 1 || state.SinceID != newest || state.UserID != anonymousAuthorID {
		t.Fatalf("sends=%d state=%+v", sends, state)
	}
}
func TestAnonymousWatcherTwoNewPostsChronological(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC1123)
	var order []string
	ids := []string{"2107676072871600472", "2107676072871600471", "2107676072871600470"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/thsottiaux/rss":
			var b strings.Builder
			b.WriteString(`<rss><channel><title>Tibo (@thsottiaux)</title>`)
			for _, id := range ids {
				fmt.Fprintf(&b, `<item><description>ordinary text</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">@thsottiaux</dc:creator><link>http://%s/thsottiaux/status/%s</link><pubDate>%s</pubDate></item>`, r.Host, id, now)
			}
			b.WriteString(`</channel></rss>`)
			w.Write([]byte(b.String()))
		case strings.HasPrefix(r.URL.Path, "/thsottiaux/status/"):
			id := strings.TrimPrefix(r.URL.Path, "/thsottiaux/status/")
			order = append(order, id)
			fmt.Fprintf(w, `{"code":200,"tweet":{"id":"%s","text":"ordinary text","author":{"screen_name":"thsottiaux","id":"1953337039510003712"}}}`, id)
		case r.URL.Path == "/v1/chat/completions":
			w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"category\":\"UNRELATED\",\"evidence\":\"ordinary text\",\"reason\":\"not reset\"}"}}]}`))
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := watchConfig{FeedURL: srv.URL + "/thsottiaux/rss", VerifyBase: srv.URL, AIBase: srv.URL, AIKey: "test", Model: "test", StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := saveWatchState(c.StatePath, watchState{UserID: anonymousAuthorID, SinceID: ids[2]}); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), srv.Client(), c); err != nil {
		t.Fatal(err)
	}
	state, _ := loadWatchState(c.StatePath)
	if strings.Join(order, ",") != ids[1]+","+ids[0] || state.SinceID != ids[0] {
		t.Fatalf("order=%v cursor=%s", order, state.SinceID)
	}
}
func TestAnonymousWatcherGapFailsClosed(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC1123)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.ReplaceAll(fmt.Sprintf(`<rss><channel><title>Tibo (@thsottiaux)</title><item><description>new reset</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">@thsottiaux</dc:creator><link>https://mirror.example/thsottiaux/status/2107676072871600472</link><pubDate>%s</pubDate></item></channel></rss>`, now), "mirror.example", r.Host)))
	}))
	defer srv.Close()
	c := watchConfig{FeedURL: srv.URL + "/thsottiaux/rss", StatePath: filepath.Join(t.TempDir(), "state.json")}
	if err := saveWatchState(c.StatePath, watchState{UserID: anonymousAuthorID, SinceID: "2107676072871600470"}); err != nil {
		t.Fatal(err)
	}
	if err := runWatchTick(context.Background(), srv.Client(), c); err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("expected gap error, got %v", err)
	}
	state, _ := loadWatchState(c.StatePath)
	if state.SinceID != "2107676072871600470" {
		t.Fatal("cursor advanced")
	}
}
