package tasks

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/kingjethro999/goo/config"
	_ "github.com/mattn/go-sqlite3"
)

const taskSchema = `
CREATE TABLE IF NOT EXISTS tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'todo',
    priority    TEXT NOT NULL DEFAULT 'medium',
    tags        TEXT DEFAULT '[]',
    due_date    DATETIME,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    project     TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS task_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    content    TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_status   ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_due      ON tasks(due_date);
`

type TaskStore struct {
	db *sql.DB
}

type Task struct {
	ID          int
	Title       string
	Description string
	Status      string
	Priority    string
	Tags        []string
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Project     string
}

func (t Task) IsOverdue() bool {
	if t.DueDate == nil || t.Status == "done" {
		return false
	}
	return time.Now().After(*t.DueDate)
}

func NewTaskStore() (*TaskStore, error) {
	dbPath := filepath.Join(config.GooConfigDir(), "tasks.db")
	db, err := sql.Open("sqlite3", dbPath+"?_fk=1")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(taskSchema); err != nil {
		return nil, fmt.Errorf("failed to init task schema: %w", err)
	}
	return &TaskStore{db: db}, nil
}

func (s *TaskStore) Insert(title, desc, priority, project string, tags []string, due *time.Time) (*Task, error) {
	now := time.Now()
	if priority == "" {
		priority = "medium"
	}
	tagsJSON, _ := json.Marshal(tags)
	if len(tags) == 0 {
		tagsJSON = []byte("[]")
	}

	res, err := s.db.Exec(
		`INSERT INTO tasks (title, description, status, priority, tags, due_date, created_at, updated_at, project)
		 VALUES (?, ?, 'todo', ?, ?, ?, ?, ?, ?)`,
		title, desc, priority, string(tagsJSON), due, now, now, project,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Task{
		ID:          int(id),
		Title:       title,
		Description: desc,
		Status:      "todo",
		Priority:    priority,
		Tags:        tags,
		DueDate:     due,
		CreatedAt:   now,
		UpdatedAt:   now,
		Project:     project,
	}, nil
}

func (s *TaskStore) UpdateStatus(id int, status string) error {
	_, err := s.db.Exec(`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), id)
	return err
}

func (s *TaskStore) List(where string, args []any) ([]Task, error) {
	query := `SELECT id, title, description, status, priority, tags, due_date, created_at, updated_at, project FROM tasks ` + where
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var tagsStr string
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &tagsStr, &t.DueDate, &t.CreatedAt, &t.UpdatedAt, &t.Project); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tagsStr), &t.Tags)
		tasks = append(tasks, t)
	}
	return tasks, nil
}
