package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/autokeren/payfox/internal/model"
)

const DefaultBaseURL = "https://api.meta.ai/v1"

const (
	ModelSpark       = "muse-spark-1.3"
	ModelWonder      = "muse-wonder-1.0"
	ModelImage       = "muse-image-1.0"
	ModelVoice       = "muse-voice-transcribe-1.0"
	ModelSAM         = "sam-3.1"
	ModelSparkCheap  = "muse-spark-1.3-contributor"
)

type Client struct {
	BaseURL   string
	APIKey    string
	HTTP      *http.Client
	LastUsage *model.Usage
}

func New(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 180 * time.Second},
	}
}

type CompleteOptions struct {
	Messages []model.Message
	Tools    []model.ToolDefinition
	MaxTokens int
}

func (c *Client) post(ctx context.Context, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("meta api %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	return json.Unmarshal(raw, out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Complete sends a chat completion. Muse Spark is a reasoning model:
// it burns reasoning_tokens before content — pass MaxTokens generously
// (1200+ for tool-driven turns, 4000+ for content generation).
func (c *Client) Complete(ctx context.Context, opts CompleteOptions) (*model.Response, error) {
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 4000
	}
	payload := map[string]any{
		"model":      ModelSpark,
		"messages":   opts.Messages,
		"max_tokens": opts.MaxTokens,
	}
	if len(opts.Tools) > 0 {
		payload["tools"] = opts.Tools
	}

	// API shape: {choices:[{message:{content,tool_calls},finish_reason}], usage}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content   string           `json:"content"`
				ToolCalls []model.ToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage model.Usage `json:"usage"`
	}
	if err := c.post(ctx, "/chat/completions", payload, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("empty choices from meta api")
	}
	out := model.Response{
		Content:      envelope.Choices[0].Message.Content,
		ToolCalls:    envelope.Choices[0].Message.ToolCalls,
		FinishReason: envelope.Choices[0].FinishReason,
		Usage:        envelope.Usage,
	}
	u := out.Usage
	c.LastUsage = &u
	return &out, nil
}

// Verify checks the key and model availability with a minimal live call.
func (c *Client) Verify(ctx context.Context) error {
	var out struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("verify %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	found := false
	for _, m := range out.Data {
		if m.ID == ModelSpark {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%s not in model list", ModelSpark)
	}
	return nil
}

// ReasoningTokens returns reasoning tokens from the last completion.
func (c *Client) ReasoningTokens() int {
	if c.LastUsage == nil {
		return 0
	}
	return c.LastUsage.ReasoningTokens
}
