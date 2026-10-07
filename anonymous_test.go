package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const rssFixture = `<?xml version="1.0"?><rss><channel><title>Tibo (@thsottiaux)</title><item><title>reset processed</title><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">@thsottiaux</dc:creator><description><![CDATA[<p>the reset has been processed</p>]]></description><link>https://nitter.example/thsottiaux/status/2107676072871600470</link><pubDate>Wed, 07 Oct 2026 12:00:00 GMT</pubDate></item></channel></rss>`

func TestAnonymousRSSParsesVerifiedRecentPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/thsottiaux/rss" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(strings.ReplaceAll(rssFixture, "https://nitter.example", "http://"+r.Host)))
	}))
	defer srv.Close()
	posts, err := anonymousPosts(context.Background(), srv.Client(), srv.URL+"/thsottiaux/rss", time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].ID != "2107676072871600470" || posts[0].AuthorID != "1953337039510003712" || posts[0].Text != "the reset has been processed" {
		t.Fatalf("posts=%+v", posts)
	}
}
func TestAnonymousRSSRealMirrorShape(t *testing.T) {
	feed := `<rss xmlns:dc="http://purl.org/dc/elements/1.1/"><channel><title>Tibo / @thsottiaux</title><item><title>R to @thsottiaux: The reset has been processed</title><dc:creator>@thsottiaux</dc:creator><description><![CDATA[<p>The reset has been processed</p><hr/><blockquote>quoted third party</blockquote>]]></description><link>http://nitter.meowing.monster/thsottiaux/status/2107676072871600470#m</link><pubDate>Wed, 07 Oct 2026 03:35:09 GMT</pubDate></item></channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.ReplaceAll(feed, "nitter.meowing.monster", r.Host)))
	}))
	defer srv.Close()
	posts, err := anonymousPosts(context.Background(), srv.Client(), srv.URL+"/thsottiaux/rss", time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Text != "The reset has been processed" {
		t.Fatalf("posts=%+v", posts)
	}
}
func TestAnonymousRSSAllowsOldItemsWhenLatestFresh(t *testing.T) {
	feed := strings.Replace(rssFixture, "07 Oct 2026 12:00:00", "05 Oct 2026 12:00:00", 1)
	// Older items in a finite feed are expected; freshness is measured at the head.
	feed = strings.Replace(feed, "</channel>", `<item><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">@thsottiaux</dc:creator><description>new</description><link>https://nitter.example/thsottiaux/status/2107676653895967106</link><pubDate>Wed, 07 Oct 2026 12:00:00 GMT</pubDate></item></channel>`, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.ReplaceAll(feed, "https://nitter.example", "http://"+r.Host)))
	}))
	defer srv.Close()
	if _, err := anonymousPosts(context.Background(), srv.Client(), srv.URL+"/thsottiaux/rss", time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
}
func TestAnonymousRSSFailsClosed(t *testing.T) {
	cases := map[string]string{
		"empty":     "",
		"challenge": "<html>Making sure you're not a bot</html>",
		"foreign":   "<?xml version=\"1.0\"?><rss><channel><title>Other (@other)</title><item><description>reset</description><link>https://nitter.example/other/status/2107676072871600470</link><pubDate>Wed, 07 Oct 2026 12:00:00 GMT</pubDate></item></channel></rss>",
		"stale":     strings.Replace(rssFixture, "07 Oct 2026", "01 Sep 2026", 1),
		"no-items":  "<?xml version=\"1.0\"?><rss><channel><title>Tibo (@thsottiaux)</title></channel></rss>",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer srv.Close()
			if _, err := anonymousPosts(context.Background(), srv.Client(), srv.URL+"/thsottiaux/rss", time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)); err == nil {
				t.Fatal("accepted invalid feed")
			}
		})
	}
}
