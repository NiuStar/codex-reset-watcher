package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type privateConfig struct {
	AIBase     string `json:"sub2api_base_url"`
	AIKey      string `json:"sub2api_api_key"`
	Model      string `json:"sub2api_model"`
	AppID      string `json:"feishu_app_id"`
	AppSecret  string `json:"feishu_app_secret"`
	ChatID     string `json:"feishu_home_channel"`
	Domain     string `json:"feishu_domain"`
	FeedURL    string `json:"anonymous_rss_url"`
	VerifyBase string `json:"anonymous_verify_base_url"`
}

func loadPrivateConfig(path string) (watchConfig, error) {
	var c watchConfig
	if path == "" {
		return c, errors.New("private config path missing")
	}
	info, err := os.Stat(path)
	if err != nil {
		return c, fmt.Errorf("private config not accessible: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return c, errors.New("private config must be mode 0600")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("private config unreadable: %w", err)
	}
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	if err = dec.Decode(&raw); err != nil {
		return c, errors.New("private config invalid JSON")
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return c, errors.New("private config trailing JSON")
	}
	// The standard JSON decoder accepts duplicate keys; reject them with a token pass.
	tok := json.NewDecoder(bytes.NewReader(b))
	if _, err = tok.Token(); err != nil {
		return c, errors.New("private config invalid")
	}
	seen := map[string]bool{}
	for tok.More() {
		key, e := tok.Token()
		if e != nil {
			return c, errors.New("private config invalid")
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return c, errors.New("private config duplicate or invalid key")
		}
		seen[name] = true
		var skip json.RawMessage
		if tok.Decode(&skip) != nil {
			return c, errors.New("private config invalid value")
		}
	}
	var p privateConfig
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&p); err != nil {
		return c, errors.New("private config unsupported fields")
	}
	if p.AIKey == "" || p.AppID == "" || p.AppSecret == "" || !strings.HasPrefix(p.ChatID, "oc_") || len(p.ChatID) <= 3 || strings.ContainsAny(p.ChatID, " \t\r\n") || p.FeedURL == "" || p.VerifyBase == "" {
		return c, errors.New("private config missing required settings")
	}
	c.AIBase = p.AIBase
	if c.AIBase == "" {
		c.AIBase = "https://sub2api.yjkj02.com"
	}
	c.Model = p.Model
	if c.Model == "" {
		c.Model = "gpt-5.6-sol"
	}
	c.AIKey = p.AIKey
	c.AppID = p.AppID
	c.AppSecret = p.AppSecret
	c.ChatID = p.ChatID
	c.FeedURL = p.FeedURL
	c.VerifyBase = p.VerifyBase
	switch p.Domain {
	case "", "feishu":
		c.FeishuBase = "https://open.feishu.cn"
	case "lark":
		c.FeishuBase = "https://open.larksuite.com"
	default:
		return watchConfig{}, errors.New("private config invalid Feishu domain")
	}
	return c, nil
}
