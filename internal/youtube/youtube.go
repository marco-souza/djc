package youtube

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/shared"

	yt "github.com/lrstanley/go-ytdlp"
)

type DownloadProgress struct {
	Name      string
	Format    string
	FilePath  string
	Percent   int
	Completed bool
}

func DownloadAudio(url string, ext string, tr *shared.TimeRange, cfg *config.Config) error {
	_, err := DownloadAudioWithProgress(context.TODO(), url, ext, tr, cfg, nil)
	return err
}

func DownloadAudioWithProgress(
	ctx context.Context,
	url string,
	ext string,
	tr *shared.TimeRange,
	cfg *config.Config,
	onProgress func(DownloadProgress),
) (*yt.Result, error) {
	if len(ext) == 0 {
		ext = cfg.AudioFormat
	}

	outputPath := filepath.Join(cfg.DownloadDir, cfg.OutputTemplate)

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

	if onProgress != nil {
		dl.ProgressFunc(time.Second, func(update yt.ProgressUpdate) {
			format := strings.ToLower(ext)
			if parsedExt := strings.TrimPrefix(strings.ToLower(filepath.Ext(update.Filename)), "."); parsedExt != "" {
				format = parsedExt
			}

			progress := DownloadProgress{
				Percent:   int(update.Percent()),
				Completed: update.Status.IsCompletedType(),
				FilePath:  update.Filename,
				Format:    format,
			}
			if update.Info != nil && update.Info.Title != nil {
				progress.Name = *update.Info.Title
			}
			onProgress(progress)
		})
	}

	proc, err := dl.Run(ctx, url)
	if err != nil {
		if proc != nil {
			return proc, fmt.Errorf("%s - %s", err, proc.Stderr)
		}
		return nil, err
	}

	fmt.Println(proc.Stdout)
	return proc, nil
}

func init() {
	yt.MustInstall(context.Background(), &yt.InstallOptions{})
}
