import os

files = {
    "tools/tasks/store.go": """package tasks

import (
	"database/sql"
	"time"
)

type TaskStore struct {
	db *sql.DB
}

type Task struct {
	ID          int
	Title       string
	Description string
	Status      string
	Priority    string
	Tags        string
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Project     string
}

func NewTaskStore() (*TaskStore, error) {
	// Dummy implementation for now to satisfy build
	return &TaskStore{}, nil
}
""",
    "tools/tasks/manager.go": """package tasks

import (
	"time"
)

type Manager struct{ store *TaskStore }

func NewManager() *Manager {
	store, _ := NewTaskStore()
	return &Manager{store: store}
}

func (m *Manager) Add(title, desc, priority, project string, tags []string, due *time.Time) (*Task, error) {
	return &Task{ID: 1, Title: title}, nil
}

type TaskFilters struct {
	Status   string
	Priority string
	Project  string
	Tag      string
	Overdue  bool
	Due      *time.Time
}

func (m *Manager) List(filters TaskFilters) ([]Task, error) {
	return []Task{}, nil
}

func (m *Manager) Complete(id int) error {
	return nil
}

type TaskStats struct {
	Total      int
	Todo       int
	InProgress int
	Done       int
	Overdue    int
	ByPriority map[string]int
}

func (m *Manager) Stats() (TaskStats, error) {
	return TaskStats{}, nil
}
""",
    "tools/github/client.go": """package github

import (
	"net/http"
	"time"
)

type Client struct {
	http     *http.Client
	baseURL  string
	username string
}

func NewClient() (*Client, error) {
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second},
		baseURL: "https://api.github.com",
	}, nil
}
""",
    "tools/github/stats.go": """package github

type ContributionStats struct {
	Username      string
	Commits       int
	PRsOpened     int
	PRsMerged     int
	PRsReviewed   int
	IssuesClosed  int
	CurrentStreak int
}

func (c *Client) GetContributionStats(username string) (*ContributionStats, error) {
	return &ContributionStats{Username: username}, nil
}
""",
    "tools/github/prs.go": """package github

type PullRequest struct {
	Number int
	Title  string
}

type Issue struct {
	Number int
	Title  string
}

func (c *Client) GetMyPRs(repo string) ([]PullRequest, error) {
	return []PullRequest{}, nil
}
"""
}

for path, content in files.items():
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write(content)
    print(f"Wrote {path}")
