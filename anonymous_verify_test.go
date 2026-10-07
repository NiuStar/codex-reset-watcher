package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyAnonymousPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"tweet":{"id":"2107676072871600470","text":"the reset has been processed","author":{"screen_name":"thsottiaux","id":"1953337039510003712"}}}`))
	}))
	defer srv.Close()
	p := xPost{ID: "2107676072871600470", Text: "the reset has been processed", AuthorID: anonymousAuthorID}
	if err := verifyAnonymousPost(context.Background(), srv.Client(), srv.URL, p); err != nil {
		t.Fatal(err)
	}
}
func TestVerifyAnonymousPostRejectsMismatch(t *testing.T) {
	for name, payload := range map[string]string{
		"wrong-text":   `{"code":200,"tweet":{"id":"2107676072871600470","text":"other","author":{"screen_name":"thsottiaux","id":"1953337039510003712"}}}`,
		"wrong-author": `{"code":200,"tweet":{"id":"2107676072871600470","text":"the reset has been processed","author":{"screen_name":"other","id":"1953337039510003712"}}}`,
		"missing-id":   `{"code":200,"tweet":{"text":"the reset has been processed","author":{"screen_name":"thsottiaux","id":"1953337039510003712"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(payload)) }))
			defer srv.Close()
			if err := verifyAnonymousPost(context.Background(), srv.Client(), srv.URL, xPost{ID: "2107676072871600470", AuthorID: anonymousAuthorID, Text: "the reset has been processed"}); err == nil {
				t.Fatal("accepted mismatch")
			}
		})
	}
}

var _ json.RawMessage
