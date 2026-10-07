package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// loadHermesFeishu reads only the Feishu settings from the local Hermes runtime.
// It never writes credentials to the repository or to watcher state.
func loadHermesFeishu(path string) (watchConfig, error) {
	var c watchConfig
	if path == "" {
		return c, errors.New("Hermes Feishu config path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return c, fmt.Errorf("Hermes Feishu config inaccessible: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return c, errors.New("Hermes Feishu config must be a private regular file (0600)")
	}
	f, err := os.Open(path)
	if err != nil {
		return c, fmt.Errorf("Hermes Feishu config inaccessible: %w", err)
	}
	defer f.Close()
	seen := map[string]bool{}
	fields := map[string]string{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !strings.HasPrefix(key, "FEISHU_") {
			continue
		}
		switch key {
		case "FEISHU_APP_ID", "FEISHU_APP_SECRET", "FEISHU_HOME_CHANNEL", "FEISHU_DOMAIN":
		default:
			continue
		}
		if seen[key] {
			return c, fmt.Errorf("Hermes Feishu config has duplicate %s", key)
		}
		seen[key] = true
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		fields[key] = value
	}
	if err := scan.Err(); err != nil {
		return c, fmt.Errorf("reading Hermes Feishu config: %w", err)
	}
	c.AppID = fields["FEISHU_APP_ID"]
	c.AppSecret = fields["FEISHU_APP_SECRET"]
	c.ChatID = fields["FEISHU_HOME_CHANNEL"]
	if c.AppID == "" || c.AppSecret == "" || !strings.HasPrefix(c.ChatID, "oc_") || len(c.ChatID) <= 3 || strings.ContainsAny(c.ChatID, " \t\r\n") {
		return watchConfig{}, errors.New("Hermes Feishu credentials or home chat missing/invalid")
	}
	switch fields["FEISHU_DOMAIN"] {
	case "feishu", "":
		c.FeishuBase = "https://open.feishu.cn"
	case "lark":
		c.FeishuBase = "https://open.larksuite.com"
	default:
		return watchConfig{}, errors.New("unsupported Hermes FEISHU_DOMAIN")
	}
	return c, nil
}
