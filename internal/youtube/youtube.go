package youtube

import (
	"context"
	"fmt"
	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/shared"
	"strings"

	yt "github.com/lrstanley/go-ytdlp"
)

func DownloadAudio(url string, ext string, tr *shared.TimeRange, cfg *config.Config) error {
	if len(ext) == 0 {
		ext = cfg.AudioFormat
	}

	// Build full output path with download directory
	outputPath := cfg.DownloadDir + "/" + cfg.OutputTemplate

	// download builder
	dl := yt.New().
		ExtractAudio().
		AudioFormat(ext).
		AudioQuality(cfg.AudioQuality).
		Output(outputPath)

	if strings.Contains(url, "/playlist") {
		dl.YesPlaylist()
	}

	if tr != nil {
		fmt.Println("Time range:", tr.String())
		dl.
			DownloadSections(tr.String()).
			ForceKeyframesAtCuts()
	}

	// execute
	proc, err := dl.Run(context.TODO(), url)
	if err != nil {
		return fmt.Errorf("%s - %s", err, proc.Stderr)
	}

	fmt.Println(proc.Stdout)
	return nil
}

func init() {
	yt.MustInstall(context.TODO(), &yt.InstallOptions{})
}
