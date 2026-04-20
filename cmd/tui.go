package cmd

import (
	"fmt"
	"os"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/library"
	"marco-souza/djc/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(tuiCmd)
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch DJ Companion TUI",
	Long:  `Launches an interactive terminal UI to manage and download songs.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		repo, err := library.Open(cfg.DatabasePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer repo.Close()

		program := tea.NewProgram(tui.NewModel(repo, cfg), tea.WithAltScreen())
		if _, err := program.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}
