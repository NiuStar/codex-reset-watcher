package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestXPostsMalformedEmptyResponseNotNoNewPosts(t *testing.T) {
	for _, body := range []string{`{}`, `{"meta":{}}`, `{"meta":{"result_count":1}}`, `{"meta":{"result_count":0,"next_token":"more"}}`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer srv.Close()
			if _, err := xPosts(context.Background(), srv.Client(), watchConfig{XBase: srv.URL, XToken: "x"}, "42", "10"); err == nil {
				t.Fatal("malformed empty response treated as no posts")
			}
		})
	}
}
