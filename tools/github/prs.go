package github

import "fmt"

type PullRequest struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Draft     bool   `json:"draft"`
}

type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
}

// Repo holds basic repository info.
type Repo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Language    string `json:"language"`
	Stars       int    `json:"stargazers_count"`
	Forks       int    `json:"forks_count"`
	Private     bool   `json:"private"`
	UpdatedAt   string `json:"updated_at"`
}

// GetMyPRs returns open pull requests authored by the configured user.
// If repo is empty, searches across all repos the user has access to.
func (c *Client) GetMyPRs(repo string) ([]PullRequest, error) {
	username := c.username
	if username == "" {
		var me struct {
			Login string `json:"login"`
		}
		if err := c.get("/user", &me); err != nil {
			return nil, fmt.Errorf("could not determine GitHub user: %w", err)
		}
		username = me.Login
	}

	query := fmt.Sprintf("type:pr+state:open+author:%s", username)
	if repo != "" {
		query += "+repo:" + repo
	}

	var result struct {
		Items []PullRequest `json:"items"`
	}
	path := fmt.Sprintf("/search/issues?q=%s&sort=updated&order=desc&per_page=20", query)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetRepos returns public repos for the given username, sorted by last updated.
func (c *Client) GetRepos(username string, limit int) ([]Repo, error) {
	if username == "" {
		var me struct {
			Login string `json:"login"`
		}
		if err := c.get("/user", &me); err != nil {
			return nil, fmt.Errorf("could not determine GitHub user: %w", err)
		}
		username = me.Login
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	path := fmt.Sprintf("/users/%s/repos?sort=updated&direction=desc&per_page=%d", username, limit)
	var repos []Repo
	if err := c.get(path, &repos); err != nil {
		return nil, err
	}
	return repos, nil
}
