package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadHermesFeishu(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte("# Hermes messaging\nFEISHU_APP_ID=cli_fixture\nFEISHU_APP_SECRET=secret_fixture\nFEISHU_HOME_CHANNEL=oc_fixture\nFEISHU_DOMAIN=lark\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadHermesFeishu(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.AppID != "cli_fixture" || c.AppSecret != "secret_fixture" || c.ChatID != "oc_fixture" || c.FeishuBase != "https://open.larksuite.com" {
		t.Fatalf("wrong fields: app set=%t secret set=%t chat=%q base=%q", c.AppID != "", c.AppSecret != "", c.ChatID, c.FeishuBase)
	}
}

func TestLoadHermesFeishuFailsClosed(t *testing.T) {
	cases := map[string]string{
		"missing-home":     "FEISHU_APP_ID=cli_test\nFEISHU_APP_SECRET=secret_test\nFEISHU_DOMAIN=feishu\n",
		"invalid-home":     "FEISHU_APP_ID=cli_test\nFEISHU_APP_SECRET=secret_test\nFEISHU_HOME_CHANNEL=ou_user\nFEISHU_DOMAIN=feishu\n",
		"invalid-domain":   "FEISHU_APP_ID=cli_test\nFEISHU_APP_SECRET=secret_test\nFEISHU_HOME_CHANNEL=oc_test\nFEISHU_DOMAIN=example.com\n",
		"duplicate-secret": "FEISHU_APP_ID=cli_test\nFEISHU_APP_SECRET=secret_test\nFEISHU_APP_SECRET=other\nFEISHU_HOME_CHANNEL=oc_test\nFEISHU_DOMAIN=feishu\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(p, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadHermesFeishu(p); err == nil {
				t.Fatal("accepted invalid Hermes Feishu config")
			} else if strings.Contains(err.Error(), "secret_test") {
				t.Fatal("secret leaked in error")
			}
		})
	}
	t.Run("world-readable", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(p, []byte("FEISHU_APP_ID=a"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadHermesFeishu(p); err == nil {
			t.Fatal("accepted world-readable credentials")
		}
	})
	t.Run("absent", func(t *testing.T) {
		if _, err := loadHermesFeishu(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("accepted absent config")
		}
	})
}
