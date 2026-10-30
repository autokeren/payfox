package qloo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const BaseURL = "https://hackathon.api.qloo.com"

type Client struct {
	APIKey string
	HTTP   *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		HTTP:   &http.Client{Timeout: 20 * time.Second},
	}
}

// SearchResult is a single entity from Qloo search.
type SearchResult struct {
	Name       string  `json:"name"`
	EntityID   string  `json:"entity_id"`
	Types      []string `json:"types"`
	Popularity float64 `json:"popularity"`
}

// InsightEntity is a recommendation result from the Insights API.
type InsightEntity struct {
	Name       string  `json:"name"`
	EntityID   string  `json:"entity_id"`
	Subtype    string  `json:"subtype"`
	Score      float64 `json:"score"`
}

func (c *Client) do(method, path string, params url.Values) ([]byte, error) {
	u := BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qloo %s %d: %s", path, resp.StatusCode, string(raw))
	}
	return raw, nil
}

// Search finds Qloo entities by name (artists, movies, brands, etc).
func (c *Client) Search(query string) ([]SearchResult, error) {
	params := url.Values{"query": {query}}
	raw, err := c.do("GET", "/search", params)
	if err != nil {
		return nil, err
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// Insights returns taste-based recommendations.
// filterType is like "urn:entity:brand" or "urn:entity:movie".
// entityIDs are Qloo entity IDs used as taste signals.
func (c *Client) Insights(filterType string, entityIDs []string) ([]InsightEntity, error) {
	if len(entityIDs) == 0 {
		return nil, fmt.Errorf("at least one entity ID required")
	}
	params := url.Values{
		"filter.type": {filterType},
	}
	for _, id := range entityIDs {
		params.Add("signal.interests.entities", id)
	}
	raw, err := c.do("GET", "/v2/insights", params)
	if err != nil {
		return nil, err
	}
	var out struct {
		Success bool `json:"success"`
		Results struct {
			Entities []InsightEntity `json:"entities"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Results.Entities, nil
}

// RecommendFlow is the high-level "taste recommendation" used by PayFox:
// given what the user likes (freeform text), return brand/movie/etc recommendations.
func (c *Client) RecommendFlow(likes string, filterType string) (string, error) {
	// Step 1: search for entities matching the user's taste
	results, err := c.Search(likes)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", fmt.Errorf("no entities found for '%s'", likes)
	}
	// Take the top 2 entities as signals
	var ids []string
	for i, r := range results {
		if i >= 2 {
			break
		}
		ids = append(ids, r.EntityID)
	}

	// Step 2: get insights based on those taste signals
	entities, err := c.Insights(filterType, ids)
	if err != nil {
		return "", err
	}
	if len(entities) == 0 {
		return "", fmt.Errorf("no recommendations for '%s' (filter: %s)", likes, filterType)
	}

	// Build a readable recommendation
	var out string
	out = fmt.Sprintf("Taste profile: %s\n\nRecommendations:\n", likes)
	for i, e := range entities {
		if i >= 5 {
			break
		}
		out += fmt.Sprintf("%d. %s (relevance: %.1f%%)\n", i+1, e.Name, e.Score*100)
	}
	return out, nil
}
