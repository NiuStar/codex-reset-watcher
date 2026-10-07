package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func samePostText(a, b string) bool {
	return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
}
func verifyAnonymousPost(ctx context.Context, client *http.Client, base string, p xPost) error {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"))) || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return errors.New("anonymous verify origin invalid")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/thsottiaux/status/"+p.ID, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("anonymous verify HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Code  int `json:"code"`
		Tweet struct {
			ID     json.Number `json:"id"`
			Text   string      `json:"text"`
			Author struct {
				ScreenName string `json:"screen_name"`
				ID         string `json:"id"`
			} `json:"author"`
		} `json:"tweet"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return errors.New("anonymous verify JSON invalid")
	}
	if payload.Code != 200 || payload.Tweet.ID.String() != p.ID || payload.Tweet.Author.ScreenName != "thsottiaux" || payload.Tweet.Author.ID != anonymousAuthorID || !samePostText(payload.Tweet.Text, p.Text) {
		return errors.New("anonymous verify author/id/text mismatch")
	}
	return nil
}
