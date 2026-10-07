package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestXPostsLongTextAndRepostContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("post.fields") != "text,created_at,note_post" || r.URL.Query().Get("expansions") != "author_id,referenced_posts" {
			t.Errorf("invalid post.fields/expansions: %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"data":[{"id":"12","author_id":"42","text":"truncated…","note_post":{"text":"the reset has been processed"}}],"meta":{"result_count":1}}`))
	}))
	defer srv.Close()
	posts, err := xPosts(context.Background(), srv.Client(), watchConfig{XBase: srv.URL, XToken: "x"}, "42", "10")
	if err != nil || len(posts) != 1 || posts[0].Text != "the reset has been processed" {
		t.Fatalf("posts=%+v err=%v", posts, err)
	}
}
func TestXPostsRejectsRepost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"12","author_id":"42","text":"a repost","referenced_posts":[{"type":"retweeted","id":"9"}]}],"meta":{"result_count":1}}`))
	}))
	defer srv.Close()
	if _, err := xPosts(context.Background(), srv.Client(), watchConfig{XBase: srv.URL, XToken: "x", StatePath: filepath.Join(t.TempDir(), "state.json")}, "42", "10"); err == nil {
		t.Fatal("repost accepted")
	}
}
