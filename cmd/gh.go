package cmd

import (
	"fmt"

	"github.com/kingjethro999/goo/tools/github"
	"github.com/spf13/cobra"
)

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "GitHub integration",
}

var ghPrsCmd = &cobra.Command{
	Use:   "prs",
	Short: "List your pull requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := github.NewClient()
		if err != nil {
			return err
		}
		prs, err := client.GetMyPRs("")
		if err != nil {
			return err
		}
		if len(prs) == 0 {
			fmt.Println("No open pull requests.")
			return nil
		}
		for _, pr := range prs {
			fmt.Printf("#%d: %s\n", pr.Number, pr.Title)
		}
		return nil
	},
}

var ghStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show contribution stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := github.NewClient()
		if err != nil {
			return err
		}
		stats, err := client.GetContributionStats("")
		if err != nil {
			return err
		}
		fmt.Printf("Commits: %d\nPRs Opened: %d\nStreak: %d days\n",
			stats.Commits, stats.PRsOpened, stats.CurrentStreak)
		return nil
	},
}

func init() {
	ghCmd.AddCommand(ghPrsCmd)
	ghCmd.AddCommand(ghStatsCmd)
}
