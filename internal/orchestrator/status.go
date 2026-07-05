package orchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/internal/agent"
)

type TaskStatus struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"` // running, done, failed, stopped
	Scope     string `json:"scope"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`
}

type LogEntry struct {
	TS     time.Time `json:"ts"`
	Agent  string    `json:"agent"`
	Level  string    `json:"level"`
	Msg    string    `json:"msg"`
	Tool   string    `json:"tool,omitempty"`
	Status string    `json:"status,omitempty"`
	Scope  string    `json:"scope,omitempty"`
	Error  string    `json:"error,omitempty"`
}

type StatusManager struct {
	mu        sync.RWMutex
	activeRun string
	tasks     map[string]*TaskStatus
	cancels   map[string]context.CancelFunc
	allCancel context.CancelFunc
}

var GlobalStatus = NewStatusManager()

func NewStatusManager() *StatusManager {
	return &StatusManager{
		tasks:   make(map[string]*TaskStatus),
		cancels: make(map[string]context.CancelFunc),
	}
}

func getLogsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".goo", "logs")
}

func (sm *StatusManager) StartRun(ctx context.Context, sessionID string, tasks []Task) (context.Context, string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sessionID == "" {
		sessionID = time.Now().Format("2006-01-02T15-04-05")
	}
	sm.activeRun = sessionID
	sm.tasks = make(map[string]*TaskStatus)
	sm.cancels = make(map[string]context.CancelFunc)

	runCtx, cancel := context.WithCancel(ctx)
	sm.allCancel = cancel

	logDir := filepath.Join(getLogsDir(), sessionID)
	_ = os.MkdirAll(logDir, 0755)

	for _, t := range tasks {
		scopeStr := ""
		if len(t.Scope) > 0 {
			scopeStr = t.Scope[0]
			if len(t.Scope) > 1 {
				scopeStr = fmt.Sprintf("%s (+%d more)", t.Scope[0], len(t.Scope)-1)
			}
		}
		sm.tasks[t.ID] = &TaskStatus{
			TaskID:    t.ID,
			Status:    "running",
			Scope:     scopeStr,
			SessionID: sessionID,
		}
		sm.writeLog(sessionID, t.ID, LogEntry{
			TS:     time.Now(),
			Agent:  t.ID,
			Level:  "info",
			Msg:    "task_started",
			Status: "running",
			Scope:  scopeStr,
		})
	}
	return runCtx, sessionID
}

func (sm *StatusManager) RegisterCancel(taskID string, cancel context.CancelFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cancels[taskID] = cancel
}

func (sm *StatusManager) UpdateTask(sessionID string, taskID string, status string, err error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	if ts, ok := sm.tasks[taskID]; ok {
		ts.Status = status
		ts.Error = errStr
	} else {
		sm.tasks[taskID] = &TaskStatus{
			TaskID:    taskID,
			Status:    status,
			SessionID: sessionID,
			Error:     errStr,
		}
	}

	delete(sm.cancels, taskID)

	sm.writeLog(sessionID, taskID, LogEntry{
		TS:     time.Now(),
		Agent:  taskID,
		Level:  "info",
		Msg:    "task_" + status,
		Status: status,
		Error:  errStr,
	})
}

func (sm *StatusManager) writeLog(sessionID, taskID string, entry LogEntry) {
	logDir := filepath.Join(getLogsDir(), sessionID)
	_ = os.MkdirAll(logDir, 0755)
	filePath := filepath.Join(logDir, fmt.Sprintf("%s.log", taskID))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		data, _ := json.Marshal(entry)
		f.Write(data)
		f.WriteString("\n")
		f.Close()
	}
}

func (sm *StatusManager) LogTurn(sessionID, taskID string, turn agent.Turn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	msg := "turn"
	toolName := ""
	switch turn.Role {
	case "tool":
		msg = "tool_result"
	case "assistant":
		msg = "assistant_response"
	}
	sm.writeLog(sessionID, taskID, LogEntry{
		TS:    time.Now(),
		Agent: taskID,
		Level: "info",
		Msg:   msg,
		Tool:  toolName,
	})
}

func (sm *StatusManager) StopTask(taskID string) error {
	sm.mu.Lock()
	cancel, ok := sm.cancels[taskID]
	if ok {
		delete(sm.cancels, taskID)
		if ts, found := sm.tasks[taskID]; found {
			ts.Status = "stopped"
		}
	}
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("task %q not found or not running", taskID)
	}
	cancel()
	sm.writeLog(sm.activeRun, taskID, LogEntry{
		TS:     time.Now(),
		Agent:  taskID,
		Level:  "warn",
		Msg:    "task_stopped",
		Status: "stopped",
	})
	return nil
}

func (sm *StatusManager) StopAll() error {
	sm.mu.Lock()
	cancel := sm.allCancel
	for id, c := range sm.cancels {
		if ts, found := sm.tasks[id]; found {
			ts.Status = "stopped"
		}
		c()
	}
	sm.cancels = make(map[string]context.CancelFunc)
	sm.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

func (sm *StatusManager) ListStatus() []TaskStatus {
	sm.mu.RLock()
	active := sm.activeRun
	inMem := make(map[string]*TaskStatus)
	for k, v := range sm.tasks {
		copyV := *v
		inMem[k] = &copyV
	}
	sm.mu.RUnlock()

	if len(inMem) > 0 {
		return sortTaskStatuses(inMem)
	}

	logsDir := getLogsDir()
	entries, err := os.ReadDir(logsDir)
	if err != nil || len(entries) == 0 {
		return nil
	}

	var sessionDirs []string
	for _, e := range entries {
		if e.IsDir() {
			sessionDirs = append(sessionDirs, e.Name())
		}
	}
	if len(sessionDirs) == 0 {
		return nil
	}
	sort.Strings(sessionDirs)
	latestSession := sessionDirs[len(sessionDirs)-1]
	if active != "" {
		latestSession = active
	}

	sessionPath := filepath.Join(logsDir, latestSession)
	files, _ := os.ReadDir(sessionPath)
	resMap := make(map[string]*TaskStatus)

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".log" || f.Name() == "lead.log" || strings.HasPrefix(f.Name(), "mcp-") {
			continue
		}
		taskID := strings.TrimSuffix(f.Name(), ".log")
		logFile := filepath.Join(sessionPath, f.Name())
		file, err := os.Open(logFile)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		var lastStatus string = "done"
		var scope string = ""
		var lastErr string = ""
		for scanner.Scan() {
			var entry LogEntry
			if json.Unmarshal(scanner.Bytes(), &entry) == nil {
				if entry.Status != "" {
					lastStatus = entry.Status
				}
				if entry.Scope != "" {
					scope = entry.Scope
				}
				if entry.Error != "" {
					lastErr = entry.Error
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator status]: read error: %v\n", err)
		}
		file.Close()
		resMap[taskID] = &TaskStatus{
			TaskID:    taskID,
			Status:    lastStatus,
			Scope:     scope,
			SessionID: latestSession,
			Error:     lastErr,
		}
	}

	return sortTaskStatuses(resMap)
}

func sortTaskStatuses(m map[string]*TaskStatus) []TaskStatus {
	var list []TaskStatus
	for _, v := range m {
		list = append(list, *v)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].TaskID < list[j].TaskID
	})
	return list
}

func GetMaxParallelAgents() int {
	val := config.GetInt("orchestration.max_parallel_agents")
	if val <= 0 {
		return 4
	}
	return val
}
