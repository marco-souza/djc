package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"marco-souza/djc/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration file",
	Long:  `Opens the configuration file in your default editor ($EDITOR).`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath, err := config.ConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
			os.Exit(1)
		}

		// Ensure config exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Config file not found. Run 'djc init' first.\n")
			os.Exit(1)
		}

		// Get editor from environment
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}

		// Split editor command in case it has arguments
		parts := strings.Fields(editor)
		editorCmd := parts[0]
		editorArgs := append(parts[1:], configPath)

		// Open editor
		execCmd := exec.Command(editorCmd, editorArgs...)
		execCmd.Stdin = os.Stdin
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr

		if err := execCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
			os.Exit(1)
		}
	},
}
