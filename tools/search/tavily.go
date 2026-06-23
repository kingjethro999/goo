package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kingjethro999/goo/config"
)

const tavilyBaseURL = "https://api.tavily.com"

// Client is a Tavily search API client.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a Tavily client.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{}}
}

// SearchRequest is the Tavily API request body.
type SearchRequest struct {
	APIKey         string   `json:"api_key"`
	Query          string   `json:"query"`
	SearchDepth    string   `json:"search_depth"`
	MaxResults     int      `json:"max_results"`
	IncludeAnswer  bool     `json:"include_answer"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

// SearchResult is a single result from Tavily.
type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// SearchResponse is the Tavily API response.
type SearchResponse struct {
	Answer  string         `json:"answer"`
	Results []SearchResult `json:"results"`
}

// Search performs a web search using the Tavily API.
func (c *Client) Search(query string) (*SearchResponse, error) {
	apiKey, err := config.GetAPIKey("tavily")
	if err != nil {
		return nil, fmt.Errorf("Tavily key not set — run: goo config set-key tavily")
	}

	maxResults := config.GetInt("search.max_results")
	if maxResults == 0 {
		maxResults = 5
	}
	depth := config.Get("search.search_depth")
	if depth == "" {
		depth = "basic"
	}

	reqBody, err := json.Marshal(SearchRequest{
		APIKey:        apiKey,
		Query:         query,
		SearchDepth:   depth,
		MaxResults:    maxResults,
		IncludeAnswer: true,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(
		tavilyBaseURL+"/search",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Tavily API error %d: %s", resp.StatusCode, string(b))
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FormatForAI formats search results as a compact string for AI context injection.
func FormatForAI(resp *SearchResponse) string {
	if resp == nil {
		return ""
	}
	var b bytes.Buffer
	if resp.Answer != "" {
		fmt.Fprintf(&b, "AI Summary: %s\n\n", resp.Answer)
	}
	for i, r := range resp.Results {
		if i >= 5 {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Content))
	}
	return b.String()
}
