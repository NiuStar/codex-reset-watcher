package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrivateConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	body := `{"sub2api_base_url":"https://sub2api.yjkj02.com","sub2api_api_key":"test-key","sub2api_model":"gpt-5.6-sol","feishu_app_id":"cli_test","feishu_app_secret":"secret","feishu_home_channel":"oc_test","feishu_domain":"feishu","anonymous_rss_url":"https://feed.example/thsottiaux/rss","anonymous_verify_base_url":"https://api.fxtwitter.com"}`
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadPrivateConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.AIKey != "test-key" || c.ChatID != "oc_test" || c.FeedURL != "https://feed.example/thsottiaux/rss" {
		t.Fatal("missing private settings")
	}
}
func TestPrivateConfigFailsClosed(t *testing.T) {
	for name, body := range map[string]string{
		"missing-feed": `{"sub2api_api_key":"key","feishu_app_id":"app","feishu_app_secret":"secret","feishu_home_channel":"oc_test"}`,
		"duplicate":    `{"sub2api_api_key":"key","sub2api_api_key":"other","anonymous_rss_url":"https://feed.example/thsottiaux/rss","anonymous_verify_base_url":"https://api.fxtwitter.com"}`,
		"empty":        `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.json")
			os.WriteFile(p, []byte(body), 0600)
			if _, err := loadPrivateConfig(p); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
	t.Run("perms", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(p, []byte(`{}`), 0644)
		if _, err := loadPrivateConfig(p); err == nil {
			t.Fatal("accepted public config")
		}
	})
}
