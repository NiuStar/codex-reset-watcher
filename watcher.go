package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type watchConfig struct {
	XBase, XToken, FeedURL, VerifyBase, AIBase, AIKey, Model, FeishuBase, AppID, AppSecret, ChatID, StatePath string
}
type watchState struct {
	UserID          string `json:"user_id"`
	SinceID         string `json:"since_id"`
	PendingID       string `json:"pending_id,omitempty"`
	PendingCategory string `json:"pending_category,omitempty"`
	MessageID       string `json:"message_id,omitempty"`
}
type xPost struct {
	ID              string `json:"id"`
	AuthorID        string `json:"author_id"`
	Text            string `json:"text"`
	ReferencedPosts []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"referenced_posts"`
	NotePost struct {
		Text string `json:"text"`
	} `json:"note_post"`
}

func validID(s string) bool {
	if len(s) < 1 || len(s) > 19 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func newer(a, b string) bool { return len(a) > len(b) || len(a) == len(b) && a > b }
func loadWatchState(path string) (watchState, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return watchState{}, nil
	}
	if err != nil {
		return watchState{}, err
	}
	var state watchState
	if err = json.Unmarshal(b, &state); err != nil {
		return state, err
	}
	if state.UserID != "" && !validID(state.UserID) || state.SinceID != "" && !validID(state.SinceID) || state.PendingID != "" && !validID(state.PendingID) {
		return state, errors.New("invalid state ids")
	}
	if state.PendingID == "" && (state.MessageID != "" || state.PendingCategory != "") {
		return state, errors.New("orphan pending state")
	}
	if state.PendingID != "" && (state.SinceID == "" || !newer(state.PendingID, state.SinceID)) {
		return state, errors.New("invalid pending id order")
	}
	if state.SinceID != "" && state.UserID == "" {
		return state, errors.New("cursor without X user ID")
	}
	if state.UserID != "" && state.SinceID == "" {
		return state, errors.New("X user ID without cursor")
	}
	if state.PendingID != "" && state.PendingCategory == "" {
		return state, errors.New("pending category missing")
	}
	return state, nil
}
func saveWatchState(path string, s watchState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func jsonRequest(ctx context.Context, client *http.Client, method, rawurl, token string, input any, output any) error {
	var body io.Reader
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawurl, body)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s HTTP %d", req.URL.Path, resp.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(output); err != nil {
		return fmt.Errorf("%s JSON: %w", req.URL.Path, err)
	}
	return nil
}
func xUser(ctx context.Context, client *http.Client, c watchConfig) (string, error) {
	var result struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	err := jsonRequest(ctx, client, "GET", c.XBase+"/2/users/by/username/thsottiaux", c.XToken, nil, &result)
	if err != nil {
		return "", err
	}
	if len(result.Errors) > 0 || result.Data.Username != "thsottiaux" || !validID(result.Data.ID) {
		return "", errors.New("X user lookup invalid")
	}
	return result.Data.ID, nil
}
func xPosts(ctx context.Context, client *http.Client, c watchConfig, userID, since string) ([]xPost, error) {
	var all []xPost
	seen := map[string]bool{}
	token := ""
	for page := 0; page < 100; page++ {
		q := url.Values{"max_results": {"100"}, "exclude": {"retweets"}, "post.fields": {"text,created_at,note_post"}, "expansions": {"author_id,referenced_posts"}}
		if since != "" {
			q.Set("since_id", since)
		}
		if token != "" {
			q.Set("pagination_token", token)
		}
		var result struct {
			Data []xPost `json:"data"`
			Meta *struct {
				NextToken   string `json:"next_token"`
				ResultCount *int   `json:"result_count"`
			} `json:"meta"`
			Errors []json.RawMessage `json:"errors"`
		}
		err := jsonRequest(ctx, client, "GET", c.XBase+"/2/users/"+userID+"/tweets?"+q.Encode(), c.XToken, nil, &result)
		if err != nil {
			return nil, err
		}
		if result.Meta == nil {
			return nil, errors.New("X page missing meta")
		}
		if len(result.Data) == 0 && (result.Meta.ResultCount == nil || *result.Meta.ResultCount != 0 || result.Meta.NextToken != "") {
			return nil, errors.New("X empty page has invalid meta")
		}
		if len(result.Data) > 0 && (result.Meta.ResultCount == nil || *result.Meta.ResultCount != len(result.Data)) {
			return nil, errors.New("X page count missing or mismatch")
		}
		if len(result.Errors) > 0 || result.Meta.NextToken != "" && len(result.Data) == 0 {
			return nil, errors.New("X page contains errors or empty continuation")
		}
		if len(result.Data) == 0 && result.Meta.NextToken == "" && page == 0 && since == "" {
			return nil, errors.New("X initial page empty")
		}
		for _, p := range result.Data {
			if p.NotePost.Text != "" {
				p.Text = p.NotePost.Text
			}
			if !validID(p.ID) || p.AuthorID != userID || strings.TrimSpace(p.Text) == "" {
				return nil, errors.New("X post identity/text missing")
			}
			if since != "" && !newer(p.ID, since) {
				return nil, errors.New("X returned stale post")
			}
			for _, ref := range p.ReferencedPosts {
				if ref.Type == "retweeted" {
					return nil, errors.New("X returned a repost")
				}
			}
			if !seen[p.ID] {
				seen[p.ID] = true
				all = append(all, p)
			}
		}
		if result.Meta.NextToken == "" {
			sort.Slice(all, func(i, j int) bool { return newer(all[j].ID, all[i].ID) })
			return all, nil
		}
		if seen["page:"+result.Meta.NextToken] {
			return nil, errors.New("X repeated pagination token")
		}
		seen["page:"+result.Meta.NextToken] = true
		token = result.Meta.NextToken
	}
	return nil, errors.New("X pagination exceeded 100 pages")
}
func sendFeishu(ctx context.Context, client *http.Client, c watchConfig, p xPost, d Decision) (string, error) {
	var auth struct {
		Code  int    `json:"code"`
		Token string `json:"tenant_access_token"`
	}
	if err := jsonRequest(ctx, client, "POST", c.FeishuBase+"/open-apis/auth/v3/tenant_access_token/internal", "", map[string]string{"app_id": c.AppID, "app_secret": c.AppSecret}, &auth); err != nil {
		return "", err
	}
	if auth.Code != 0 || auth.Token == "" {
		return "", errors.New("Feishu token failed")
	}
	labels := map[string]string{"COMPLETED": "宣布重置已处理（不保证个人账户已到账）", "SCHEDULED": "预告将重置", "BANKED": "储备重置正在发放（不代表个人账户已有可用额度）", "ISSUE": "重置覆盖异常"}
	msg := fmt.Sprintf("【Codex 重置消息】%s\n账号：@thsottiaux\n范围：%s；用户：%s\n证据：%s\n原帖：https://x.com/thsottiaux/status/%s", labels[d.Category], d.Scope, d.Audience, d.Evidence, p.ID)
	content, _ := json.Marshal(map[string]string{"text": msg})
	var sent struct {
		Code int `json:"code"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	err := jsonRequest(ctx, client, "POST", c.FeishuBase+"/open-apis/im/v1/messages?receive_id_type=chat_id", auth.Token, map[string]string{"receive_id": c.ChatID, "msg_type": "text", "content": string(content)}, &sent)
	if err != nil {
		return "", err
	}
	if sent.Code != 0 || sent.Data.MessageID == "" {
		return "", errors.New("Feishu send not confirmed")
	}
	return sent.Data.MessageID, nil
}
func runWatchTick(ctx context.Context, client *http.Client, c watchConfig) error {
	state, err := loadWatchState(c.StatePath)
	if err != nil {
		return err
	}
	if state.PendingID != "" {
		if state.MessageID != "" {
			state.SinceID = state.PendingID
			state.PendingID = ""
			state.PendingCategory = ""
			state.MessageID = ""
			if err := saveWatchState(c.StatePath, state); err != nil {
				return err
			}
			log.Printf("Feishu confirmed post_id=%s; recovered cursor without resending", state.SinceID)
		} else {
			return fmt.Errorf("pending Feishu send for post %s: reconcile before retry", state.PendingID)
		}
	}
	id := anonymousAuthorID
	var posts []xPost
	if c.FeedURL != "" {
		posts, err = anonymousPosts(ctx, client, c.FeedURL, time.Now())
		if err != nil {
			return err
		}
	} else {
		id, err = xUser(ctx, client, c)
		if err != nil {
			return err
		}
	}
	if state.UserID != "" && state.UserID != id {
		return errors.New("X account ID changed; refusing to proceed")
	}
	if state.SinceID == "" && c.FeedURL != "" {
		if err := verifyAnonymousPost(ctx, client, c.VerifyBase, posts[0]); err != nil {
			return fmt.Errorf("anonymous baseline verification failed: %w", err)
		}
		state.UserID = id
		state.SinceID = posts[0].ID
		if err := saveWatchState(c.StatePath, state); err != nil {
			return err
		}
		log.Printf("anonymous baseline established since_id=%s (historical posts skipped)", state.SinceID)
		return nil
	}
	if state.SinceID == "" {
		// First boot only establishes a head baseline; no historical notifications.
		var head struct {
			Data   []xPost           `json:"data"`
			Errors []json.RawMessage `json:"errors"`
			Meta   *struct {
				ResultCount *int `json:"result_count"`
			} `json:"meta"`
		}
		q := url.Values{"max_results": {"5"}, "exclude": {"retweets"}, "post.fields": {"text,created_at,note_post"}, "expansions": {"author_id,referenced_posts"}}
		if err := jsonRequest(ctx, client, "GET", c.XBase+"/2/users/"+id+"/tweets?"+q.Encode(), c.XToken, nil, &head); err != nil {
			return err
		}
		if len(head.Errors) > 0 || head.Meta == nil || head.Meta.ResultCount == nil || *head.Meta.ResultCount != len(head.Data) || len(head.Data) == 0 || !validID(head.Data[0].ID) || head.Data[0].AuthorID != id || strings.TrimSpace(head.Data[0].Text) == "" && strings.TrimSpace(head.Data[0].NotePost.Text) == "" {
			return errors.New("initial X baseline invalid or empty")
		}
		for _, p := range head.Data {
			if p.NotePost.Text != "" {
				p.Text = p.NotePost.Text
			}
			if !validID(p.ID) || p.AuthorID != id || strings.TrimSpace(p.Text) == "" {
				return errors.New("initial X baseline contains invalid post")
			}
			if newer(p.ID, head.Data[0].ID) {
				return errors.New("initial X baseline not newest-first")
			}
		}
		state.UserID = id
		state.SinceID = head.Data[0].ID
		if err := saveWatchState(c.StatePath, state); err != nil {
			return err
		}
		log.Printf("X baseline established user_id=%s since_id=%s (historical posts skipped)", id, state.SinceID)
		return nil
	}
	if c.FeedURL == "" {
		posts, err = xPosts(ctx, client, c, id, state.SinceID)
		if err != nil {
			return err
		}
	} else {
		foundCursor := false
		for _, p := range posts {
			if p.ID == state.SinceID {
				foundCursor = true
				break
			}
		}
		if !foundCursor {
			return errors.New("anonymous feed cursor missing: gap or stale mirror; manual reconciliation required")
		}
		filtered := make([]xPost, 0, len(posts))
		for _, p := range posts {
			if newer(p.ID, state.SinceID) {
				filtered = append(filtered, p)
			}
		}
		// The feed is newest-first, but cursor advancement must be oldest-first.
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
		posts = filtered
	}
	state.UserID = id
	for _, p := range posts {
		if c.FeedURL != "" {
			if err := verifyAnonymousPost(ctx, client, c.VerifyBase, p); err != nil {
				return fmt.Errorf("post %s verify failed: %w", p.ID, err)
			}
		}
		d, err := classify(ctx, client, c.AIBase, c.AIKey, c.Model, Post{ID: p.ID, Author: "thsottiaux", Text: p.Text})
		if err != nil {
			return err
		}
		if d.Category == "REVIEW_REQUIRED" {
			return fmt.Errorf("post %s needs manual classification", p.ID)
		}
		switch d.Category {
		case "COMPLETED", "SCHEDULED", "BANKED", "ISSUE":
			state.PendingID = p.ID
			state.PendingCategory = d.Category
			state.MessageID = ""
			if err = saveWatchState(c.StatePath, state); err != nil {
				return err
			}
			msgID, err := sendFeishu(ctx, client, c, p, d)
			if err != nil {
				return fmt.Errorf("post %s send unconfirmed; manual reconciliation required: %w", p.ID, err)
			}
			state.MessageID = msgID
			if err = saveWatchState(c.StatePath, state); err != nil {
				return err
			}
			state.PendingID = ""
			state.PendingCategory = ""
			state.MessageID = ""
			state.SinceID = p.ID
			if err = saveWatchState(c.StatePath, state); err != nil {
				return err
			}
			log.Printf("Feishu confirmed post_id=%s category=%s message_id=%s", p.ID, d.Category, msgID)
			continue
		}
		state.SinceID = p.ID
		if err = saveWatchState(c.StatePath, state); err != nil {
			return err
		}
	}
	return nil
}
