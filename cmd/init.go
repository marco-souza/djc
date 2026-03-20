package cmd

import (
	"fmt"
	"os"

	"marco-souza/djc/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize djc configuration",
	Long:  `Creates the default configuration file in the XDG config directory and necessary directories.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if config already exists
		configPath, err := config.ConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("Configuration already exists at:\n  %s\n\nUse 'djc edit' to modify it.\n", configPath)
			return
		}

		// Create default config (this creates directories and file)
		cfg := config.DefaultConfig()
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
			os.Exit(1)
		}

		// Create download directory
		if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating download directory: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Configuration initialized!\n\n")
		fmt.Printf("Config file:\n  %s\n\n", configPath)
		fmt.Printf("Download directory:\n  %s\n\n", cfg.DownloadDir)
		fmt.Printf("Run 'djc edit' to customize settings.\n")
	},
}
