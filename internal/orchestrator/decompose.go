package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/memory"
	"github.com/kingjethro999/goo/tools/ai"
	"gopkg.in/yaml.v3"
)

type tasksFile struct {
	Tasks []Task `json:"tasks" yaml:"tasks"`
}

func LoadTasksFile(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tasks file: %w", err)
	}

	var tf tasksFile
	if err := yaml.Unmarshal(data, &tf); err == nil && len(tf.Tasks) > 0 {
		return tf.Tasks, nil
	}

	if err := json.Unmarshal(data, &tf); err == nil && len(tf.Tasks) > 0 {
		return tf.Tasks, nil
	}

	var rawTasks []Task
	if err := yaml.Unmarshal(data, &rawTasks); err == nil && len(rawTasks) > 0 {
		return rawTasks, nil
	}
	if err := json.Unmarshal(data, &rawTasks); err == nil && len(rawTasks) > 0 {
		return rawTasks, nil
	}

	return nil, fmt.Errorf("could not parse task list from %q", path)
}

func Decompose(ctx context.Context, goal string, files []string) ([]Task, error) {
	client, err := ai.NewGroqClient()
	if err != nil {
		return nil, fmt.Errorf("decompose: create ai client: %w", err)
	}

	model := config.Get("general.default_model")
	if model == "" {
		model = "openai/gpt-oss-120b"
	}

	prompt := fmt.Sprintf(`You are a task decomposition assistant.
Decompose the following user goal into independent parallel tasks.
Each task MUST touch a distinct, non-overlapping file scope. If tasks would overlap, merge them into a single task.
Return ONLY a valid JSON array of tasks with structure: [{"id": "t1", "description": "...", "scope": ["file1.go"]}]

User Goal: %s
Available Files:
%s`, goal, strings.Join(files, "\n"))

	messages := []memory.Message{
		{Role: "system", Content: "You decompose software tasks into non-overlapping JSON task arrays."},
		{Role: "user", Content: prompt},
	}

	var respBuf strings.Builder
	opts := ai.StreamOptions{
		Model: model,
	}

	_, err = client.StreamChatWithToolsEx(ctx, messages, &respBuf, nil, opts)
	if err != nil {
		return nil, fmt.Errorf("decompose model call failed: %w", err)
	}

	content := strings.TrimSpace(respBuf.String())
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var tf tasksFile
	if err := json.Unmarshal([]byte(content), &tf); err == nil && len(tf.Tasks) > 0 {
		return tf.Tasks, nil
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(content), &tasks); err == nil {
		return tasks, nil
	}

	return nil, fmt.Errorf("failed to parse tasks from model response: %s", content)
}
