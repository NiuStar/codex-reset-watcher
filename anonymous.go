package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const anonymousAuthorID = "1953337039510003712"

var rssTags = regexp.MustCompile(`<[^>]*>`)

func anonymousPosts(ctx context.Context, client *http.Client, feedURL string, now time.Time) ([]xPost, error) {
	u, err := url.Parse(feedURL)
	if err != nil || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Path != "/thsottiaux/rss" {
		return nil, errors.New("anonymous feed must be an HTTPS /thsottiaux/rss URL")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "codex-reset-watcher/anonymous-rss")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anonymous feed HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw) >= 2<<20 {
		return nil, errors.New("anonymous feed empty or too large")
	}
	var feed struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Description string `xml:"description"`
				Link        string `xml:"link"`
				Published   string `xml:"pubDate"`
				Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err = xml.Unmarshal(raw, &feed); err != nil {
		return nil, fmt.Errorf("anonymous feed invalid RSS: %w", err)
	}
	if !strings.Contains(strings.ToLower(feed.Channel.Title), "@thsottiaux") || len(feed.Channel.Items) == 0 {
		return nil, errors.New("anonymous feed identity/items invalid")
	}
	if len(feed.Channel.Items) > 100 {
		return nil, errors.New("anonymous feed exceeds item limit")
	}
	var newest time.Time
	for _, item := range feed.Channel.Items {
		published, err := time.Parse(time.RFC1123Z, item.Published)
		if err != nil {
			published, err = time.Parse(time.RFC1123, item.Published)
		}
		if err != nil || published.After(now.Add(time.Hour)) {
			return nil, errors.New("anonymous feed timestamp invalid")
		}
		if published.After(newest) {
			newest = published
		}
	}
	if newest.IsZero() || now.Sub(newest) > 72*time.Hour {
		return nil, errors.New("anonymous feed latest timestamp stale")
	}
	posts := make([]xPost, 0, len(feed.Channel.Items))
	seen := map[string]bool{}
	for _, item := range feed.Channel.Items {
		link, err := url.Parse(item.Link)
		if err != nil || (link.Scheme != "https" && link.Scheme != "http") || !strings.EqualFold(link.Host, u.Host) || link.User != nil || link.RawQuery != "" || (link.Fragment != "" && link.Fragment != "m") || item.Creator != "@thsottiaux" {
			return nil, errors.New("anonymous post link invalid")
		}
		path := strings.Split(strings.Trim(link.Path, "/"), "/")
		if len(path) != 3 || !strings.EqualFold(path[0], "thsottiaux") || path[1] != "status" || !validID(path[2]) {
			return nil, errors.New("anonymous post author or ID invalid")
		}
		published, err := time.Parse(time.RFC1123Z, item.Published)
		if err != nil {
			published, err = time.Parse(time.RFC1123, item.Published)
		}
		if err != nil || published.After(now.Add(time.Hour)) || now.Sub(published) > 30*24*time.Hour {
			return nil, errors.New("anonymous feed timestamp invalid or implausibly old")
		}
		text := strings.TrimSpace(html.UnescapeString(rssTags.ReplaceAllString(strings.SplitN(item.Description, "<hr", 2)[0], " ")))
		if text == "" || seen[path[2]] {
			return nil, errors.New("anonymous post empty or duplicated")
		}
		seen[path[2]] = true
		posts = append(posts, xPost{ID: path[2], AuthorID: anonymousAuthorID, Text: text})
	}
	sort.Slice(posts, func(i, j int) bool { return newer(posts[i].ID, posts[j].ID) })
	return posts, nil
}
