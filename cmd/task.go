package cmd

import (
	"fmt"
	"strconv"

	"github.com/kingjethro999/goo/tools/tasks"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
}

var taskAddCmd = &cobra.Command{
	Use:   "add [title]",
	Short: "Add a new task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := tasks.NewManager()
		task, err := mgr.Add(args[0], "", "medium", "", nil, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Added task #%d: %s\n", task.ID, task.Title)
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := tasks.NewManager()
		list, err := mgr.List(tasks.TaskFilters{})
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}
		for _, t := range list {
			fmt.Printf("[%d] %s (%s)\n", t.ID, t.Title, t.Status)
		}
		return nil
	},
}

var taskDoneCmd = &cobra.Command{
	Use:   "done [id]",
	Short: "Mark a task as done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task ID")
		}
		mgr := tasks.NewManager()
		if err := mgr.Complete(id); err != nil {
			return err
		}
		fmt.Printf("Task #%d marked as done.\n", id)
		return nil
	},
}

func init() {
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskDoneCmd)
}
