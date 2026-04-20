package youtube

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/shared"

	yt "github.com/lrstanley/go-ytdlp"
)

var installOnce sync.Once

// ytdlpInstallTimeout is the maximum time allowed for the yt-dlp binary installation.
// 5 minutes is generous to accommodate slow networks and CI environments.
const ytdlpInstallTimeout = 5 * time.Minute

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
	// Install yt-dlp on first use rather than at package init time so that
	// tests and unrelated commands are not forced to block on network I/O.
	installOnce.Do(func() {
		installCtx, cancel := context.WithTimeout(context.Background(), ytdlpInstallTimeout)
		defer cancel()
		yt.MustInstall(installCtx, &yt.InstallOptions{})
	})
	if len(ext) == 0 {
		ext = cfg.AudioFormat
	}

	outputPath := filepath.Join(cfg.DownloadDir, cfg.OutputTemplate)

	dl := yt.New().
		ExtractAudio().
		AudioFormat(ext).
		AudioQuality(cfg.AudioQuality).
		Output(outputPath).
		Quiet().
		NoWarnings()

	if strings.Contains(url, "/playlist") {
		dl.YesPlaylist()
	}

	if tr != nil {
		dl.
			DownloadSections(tr.String()).
			ForceKeyframesAtCuts()
	}

	if onProgress != nil {
		dl.ProgressFunc(time.Second, func(update yt.ProgressUpdate) {
			// Use the format from the output filename when available; fall back to the
			// requested extension derived from the configured audio format.
			fileExt := strings.TrimPrefix(strings.ToLower(filepath.Ext(update.Filename)), ".")
			format := strings.ToLower(ext)
			if fileExt != "" {
				format = fileExt
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

	return proc, nil
}



