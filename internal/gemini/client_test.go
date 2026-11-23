package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "test-secret-key"

func newTestClient(t *testing.T, h http.HandlerFunc, maxAttempts int) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Config{
		APIKey:      testKey,
		BaseURL:     srv.URL + "/",
		MaxAttempts: maxAttempts,
		Backoff:     time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGenerateRetriesTransientErrorsAndSkipsThoughts(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("request URL must not carry query params, got %q", r.URL.RawQuery)
		}
		if got := r.Header.Get("x-goog-api-key"); got != testKey {
			t.Errorf("api key header = %q", got)
		}
		if calls.Add(1) == 1 {
			http.Error(w, "overloaded", http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[
			{"text":"private reasoning","thought":true},
			{"text":"{\"ok\":true}"}]}}]}`)
	}, 3)

	got, err := c.Generate(context.Background(), Request{Prompt: "hi", JSONOutput: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"ok":true}` {
		t.Errorf("got %q, want thought parts excluded", got)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("calls = %d, want 2", n)
	}
}

func TestGenerateDoesNotRetryClientErrorsAndRedactsKey(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}, 3)

	_, err := c.Generate(context.Background(), Request{Prompt: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("calls = %d, want 1", n)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("error leaks API key: %v", err)
	}
}

func TestGenerateGivesUpAfterMaxAttempts(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}, 3)

	if _, err := c.Generate(context.Background(), Request{Prompt: "hi"}); err == nil {
		t.Fatal("expected error")
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("calls = %d, want 3", n)
	}
}

func TestRequestBodyCarriesSchemaAndThinking(t *testing.T) {
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"[]"}]}}]}`)
	}, 1)

	_, err := c.Generate(context.Background(), Request{
		SystemInstruction: "sys",
		Prompt:            "user",
		Schema:            map[string]any{"type": "ARRAY"},
		ThinkingLevel:     "HIGH",
	})
	if err != nil {
		t.Fatal(err)
	}
	gen := body["generationConfig"].(map[string]any)
	if gen["responseMimeType"] != "application/json" {
		t.Errorf("responseMimeType = %v", gen["responseMimeType"])
	}
	if gen["responseSchema"] == nil {
		t.Error("responseSchema missing")
	}
	think := gen["thinkingConfig"].(map[string]any)
	if think["thinkingLevel"] != "HIGH" {
		t.Errorf("thinkingLevel = %v", think["thinkingLevel"])
	}
}
