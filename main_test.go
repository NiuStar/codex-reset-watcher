package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClassifyRealPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("wrong request: %s", r.URL)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"category\":\"COMPLETED\",\"evidence\":\"the reset has been processed\",\"reason\":\"Explicitly completed\"}"}}]}`))
	}))
	defer srv.Close()
	got, err := classify(context.Background(), srv.Client(), srv.URL, "test-key", "gpt-5.6-sol", Post{ID: "2107676072871600470", Author: "thsottiaux", Text: "Therefore ... the reset has been processed. Enjoy!"})
	if err != nil || got.Category != "COMPLETED" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestClassifyRejectsInvalidEvidence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"category\":\"COMPLETED\",\"evidence\":\"all users received it\",\"reason\":\"claim\"}"}}]}`))
	}))
	defer srv.Close()
	_, err := classify(context.Background(), srv.Client(), srv.URL, "k", "m", Post{ID: "1", Author: "thsottiaux", Text: "The reset has been processed."})
	if err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatal(err)
	}
}

func TestClassifyRejectsPartialResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"{\"category\":\"COMPLETED\"}"}}]}`))
	}))
	defer srv.Close()
	_, err := classify(context.Background(), srv.Client(), srv.URL, "k", "m", Post{ID: "1", Author: "thsottiaux", Text: "reset"})
	if err == nil || !strings.Contains(err.Error(), "finish_reason") {
		t.Fatal(err)
	}
}
