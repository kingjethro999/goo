package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kingjethro999/goo/config"
)

const baseURL = "https://api.github.com"

type Client struct {
	http     *http.Client
	token    string
	username string
}

func NewClient() (*Client, error) {
	token, _ := config.GetAPIKey("github")
	username := config.Get("github.username")
	return &Client{
		http:     &http.Client{Timeout: 15 * time.Second},
		token:    token,
		username: username,
	}, nil
}

func (c *Client) get(path string, out any) error {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("GitHub API: unauthorized — run: goo config set-key github")
	}
	if resp.StatusCode == 403 {
		return fmt.Errorf("GitHub API: rate limited or forbidden")
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(b))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
