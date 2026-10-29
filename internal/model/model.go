package model

import "encoding/json"

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"` // string OR []ContentPart (multimodal)
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ContentPart is a multimodal message part (text, image, or audio).
type ContentPart struct {
	Type       string      `json:"type"` // "text" | "image_url" | "input_audio" | "video_url"
	Text       string      `json:"text,omitempty"`
	ImageURL   *ImageURL   `json:"image_url,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
	VideoURL   *VideoURL   `json:"video_url,omitempty"`
}

type VideoURL struct {
	URL string `json:"url"`
}

type InputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}


type ImageURL struct {
	URL string `json:"url"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type Usage struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	NeuronsUsed      int `json:"neurons_used,omitempty"`
	NeuronsRemaining int `json:"neurons_remaining,omitempty"`
	NeuronsQuota     int `json:"neurons_quota,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type Request struct {
	Model         string           `json:"model"`
	Messages      []Message        `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Temperature   float64          `json:"temperature,omitempty"`
	MaxTokens     int              `json:"max_tokens,omitempty"`
	Stream        bool             `json:"stream,omitempty"`
	StreamOptions *StreamOptions   `json:"stream_options,omitempty"`
}

type Response struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Usage        Usage      `json:"usage"`
	Model        string     `json:"model"`
	FinishReason string     `json:"finish_reason,omitempty"`
}
