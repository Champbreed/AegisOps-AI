// Package gemini is a small client for the Gemini generateContent REST API.
//
// The API key is sent in the x-goog-api-key header, never in the URL, so it
// cannot leak into error messages or logs that print request URLs.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production Gemini API endpoint prefix. It ends in "/".
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"
	// DefaultModel is the model every AegisOps audit uses.
	DefaultModel = "gemini-3-flash-preview"
)

// Config configures a Client. Zero values pick sensible defaults.
type Config struct {
	APIKey      string
	Model       string        // default: DefaultModel
	BaseURL     string        // default: DefaultBaseURL (overridden in tests)
	HTTPClient  *http.Client  // default: client with a 2-minute timeout
	MaxAttempts int           // default: 5
	Backoff     time.Duration // base delay, doubled after each failed attempt; default: 2s
}

// Client calls the Gemini API.
type Client struct {
	cfg Config
}

// New validates cfg and returns a Client.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("gemini: API key is empty")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 2 * time.Minute}
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.Backoff == 0 {
		cfg.Backoff = 2 * time.Second
	}
	return &Client{cfg: cfg}, nil
}

// Request describes one generation call.
type Request struct {
	// SystemInstruction sets the model's role and rules. Optional.
	SystemInstruction string
	// Prompt is the user message.
	Prompt string
	// JSONOutput asks for application/json output without a schema.
	JSONOutput bool
	// Schema, if non-nil, constrains the output and implies JSONOutput.
	Schema map[string]any
	// ThinkingLevel is "LOW", "MEDIUM" or "HIGH". Empty leaves thinking at the model default.
	ThinkingLevel string
}

// Generate sends req and returns the text of the model's answer, excluding
// thought summaries. It retries rate limits, server errors, transport failures
// and empty responses with exponential backoff. Other 4xx errors fail at once.
func (c *Client) Generate(ctx context.Context, req Request) (string, error) {
	body, err := buildBody(req)
	if err != nil {
		return "", fmt.Errorf("gemini: encoding request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.cfg.MaxAttempts; attempt++ {
		text, retryable, err := c.once(ctx, body)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !retryable {
			return "", fmt.Errorf("gemini: %w", err)
		}
		if attempt == c.cfg.MaxAttempts {
			break
		}
		delay := c.cfg.Backoff << (attempt - 1)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", fmt.Errorf("gemini: giving up after %d attempts: %w", c.cfg.MaxAttempts, lastErr)
}

func (c *Client) once(ctx context.Context, body []byte) (text string, retryable bool, err error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+c.cfg.Model+":generateContent", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.cfg.APIKey)

	resp, err := c.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		return "", true, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", true, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", retry, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}

	var parsed response
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false, fmt.Errorf("decoding response: %w", err)
	}
	text = parsed.text()
	if text == "" {
		return "", true, errors.New("response contained no text")
	}
	return text, false, nil
}

func buildBody(req Request) ([]byte, error) {
	body := map[string]any{
		"contents": []any{
			map[string]any{
				"role":  "user",
				"parts": []any{map[string]any{"text": req.Prompt}},
			},
		},
	}
	if req.SystemInstruction != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []any{map[string]any{"text": req.SystemInstruction}},
		}
	}

	gen := map[string]any{}
	if req.JSONOutput || req.Schema != nil {
		gen["responseMimeType"] = "application/json"
	}
	if req.Schema != nil {
		gen["responseSchema"] = req.Schema
	}
	if req.ThinkingLevel != "" {
		gen["thinkingConfig"] = map[string]any{"thinkingLevel": req.ThinkingLevel}
	}
	if len(gen) > 0 {
		body["generationConfig"] = gen
	}
	return json.Marshal(body)
}

type response struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (r response) text() string {
	if len(r.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		if !p.Thought {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
