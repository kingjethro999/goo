package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Goo to the latest available version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Checking and updating Goo to the latest version...")
		c := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/kingjethro999/goo/main/install.sh | bash")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin
		return c.Run()
	},
}
