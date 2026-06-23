package tasks

import (
	"strings"
	"time"
)

type Manager struct{ store *TaskStore }

func NewManager() *Manager {
	store, _ := NewTaskStore()
	return &Manager{store: store}
}

func (m *Manager) Add(title, desc, priority, project string, tags []string, due *time.Time) (*Task, error) {
	return m.store.Insert(title, desc, priority, project, tags, due)
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
	var conditions []string
	var args []any

	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}
	if filters.Priority != "" {
		conditions = append(conditions, "priority = ?")
		args = append(args, filters.Priority)
	}
	if filters.Project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, filters.Project)
	}
	if filters.Overdue {
		conditions = append(conditions, "due_date < ? AND status != 'done'")
		args = append(args, time.Now())
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	return m.store.List(where, args)
}

func (m *Manager) Complete(id int) error {
	return m.store.UpdateStatus(id, "done")
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
	all, err := m.store.List("", nil)
	if err != nil {
		return TaskStats{}, err
	}
	stats := TaskStats{ByPriority: make(map[string]int)}
	for _, t := range all {
		stats.Total++
		switch t.Status {
		case "todo":
			stats.Todo++
		case "in_progress":
			stats.InProgress++
		case "done":
			stats.Done++
		}
		if t.IsOverdue() {
			stats.Overdue++
		}
		stats.ByPriority[t.Priority]++
	}
	return stats, nil
}
