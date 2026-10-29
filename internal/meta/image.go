package meta

import (
	"context"
		"fmt"
)

// ImageResult is one generated image. Muse Image returns either a URL or b64.
type ImageResult struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

type imageResponse struct {
	Data []ImageResult `json:"data"`
}

// GenerateImage creates an image with muse-image-1.0.
func (c *Client) GenerateImage(ctx context.Context, prompt, size string) (*ImageResult, error) {
	if size == "" {
		size = "1024x1024"
	}
	payload := map[string]any{
		"model":  ModelImage,
		"prompt": prompt,
		"size":   size,
	}
	var out imageResponse
	if err := c.post(ctx, "/images/generations", payload, &out); err != nil {
		return nil, fmt.Errorf("muse-image: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("muse-image: no data in response")
	}
	return &out.Data[0], nil
}

