package github

import "fmt"

type ContributionStats struct {
	Username      string `json:"username"`
	PublicRepos   int    `json:"public_repos"`
	Followers     int    `json:"followers"`
	Following     int    `json:"following"`
	Commits       int    `json:"commits"`
	PRsOpened     int    `json:"prs_opened"`
	PRsMerged     int    `json:"prs_merged"`
	IssuesClosed  int    `json:"issues_closed"`
	CurrentStreak int    `json:"current_streak"`
}

type userProfile struct {
	Login       string `json:"login"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
}

type searchResult struct {
	TotalCount int `json:"total_count"`
}

// GetContributionStats fetches publicly available contribution data for a user
// via the GitHub Search API (no GraphQL required, works with a standard PAT).
func (c *Client) GetContributionStats(username string) (*ContributionStats, error) {
	if username == "" {
		var me userProfile
		if err := c.get("/user", &me); err != nil {
			return nil, fmt.Errorf("could not determine GitHub user: %w", err)
		}
		username = me.Login
	}

	var profile userProfile
	if err := c.get("/users/"+username, &profile); err != nil {
		return nil, err
	}

	// Commits this year
	var commits searchResult
	_ = c.get(fmt.Sprintf("/search/commits?q=author:%s&per_page=1", username), &commits)

	// Open PRs
	var openPRs searchResult
	_ = c.get(fmt.Sprintf("/search/issues?q=type:pr+state:open+author:%s&per_page=1", username), &openPRs)

	// Merged PRs
	var mergedPRs searchResult
	_ = c.get(fmt.Sprintf("/search/issues?q=type:pr+is:merged+author:%s&per_page=1", username), &mergedPRs)

	// Closed issues
	var closedIssues searchResult
	_ = c.get(fmt.Sprintf("/search/issues?q=type:issue+state:closed+author:%s&per_page=1", username), &closedIssues)

	return &ContributionStats{
		Username:     profile.Login,
		PublicRepos:  profile.PublicRepos,
		Followers:    profile.Followers,
		Following:    profile.Following,
		Commits:      commits.TotalCount,
		PRsOpened:    openPRs.TotalCount,
		PRsMerged:    mergedPRs.TotalCount,
		IssuesClosed: closedIssues.TotalCount,
	}, nil
}
