/*
Copyright © 2025 Marco Souza <marco@tremtec.com>
*/
package cmd

import (
	"fmt"
	"os"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/shared"
	"marco-souza/djc/internal/youtube"

	"github.com/spf13/cobra"
)

var (
	ext   string
	start string
	end   string
)

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringVarP(&ext, "ext", "x", "", "Specify audio extension (default: flac)")
	downloadCmd.Flags().StringVarP(&start, "start", "s", "", "Specify start time (format: HH:MM:SS.ms)")
	downloadCmd.Flags().StringVarP(&end, "end", "e", "", "Specify end time (format: HH:MM:SS.ms)")
}

var downloadCmd = &cobra.Command{
	Use:     "download <youtube-url>",
	Aliases: []string{"d"},
	Short:   "Download audio from YouTube",
	Long:    `Download audio from a YouTube URL with configurable format and time range.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		youtubeURL := args[0]
		fmt.Println("Downloading from URL:", youtubeURL)

		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		tr := shared.NewTimeRange(start, end)

		// Use flag value if provided, otherwise use config default
		format := cfg.AudioFormat
		if ext != "" {
			format = ext
		}

		if err := youtube.DownloadAudio(youtubeURL, format, tr, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}
