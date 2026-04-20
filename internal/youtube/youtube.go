package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/shared"

	yt "github.com/lrstanley/go-ytdlp"
)

// VideoInfo holds lightweight metadata for a single video (fetched without downloading).
type VideoInfo struct {
	Title    string
	URL      string
	Duration float64 // seconds; 0 if unknown
}

// FetchMetadata retrieves video or playlist metadata without downloading.
// For playlists it returns one VideoInfo per entry; for single videos it
// returns a single-element slice.
func FetchMetadata(ctx context.Context, rawURL string) ([]VideoInfo, error) {
	installOnce.Do(func() {
		installCtx, cancel := context.WithTimeout(context.Background(), ytdlpInstallTimeout)
		defer cancel()
		yt.MustInstall(installCtx, &yt.InstallOptions{})
	})

	dl := yt.New().
		DumpSingleJSON().
		NoWarnings()

	if IsPlaylistURL(rawURL) {
		dl.YesPlaylist().FlatPlaylist()
	} else {
		dl.NoPlaylist()
	}

	proc, err := dl.Run(ctx, rawURL)
	if err != nil {
		if proc != nil {
			return nil, fmt.Errorf("fetch metadata: %s - %s", err, proc.Stderr)
		}
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}

	stdout := strings.TrimSpace(proc.Stdout)
	if stdout == "" {
		return nil, fmt.Errorf("fetch metadata: no output from yt-dlp")
	}

	// --dump-single-json emits one JSON blob to stdout.
	var raw struct {
		Title      *string  `json:"title"`
		Duration   *float64 `json:"duration"`
		WebpageURL *string  `json:"webpage_url"`
		Entries    []struct {
			ID         string   `json:"id"`
			Title      *string  `json:"title"`
			Duration   *float64 `json:"duration"`
			URL        *string  `json:"url"`
			WebpageURL *string  `json:"webpage_url"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("fetch metadata: parse JSON: %w", err)
	}

	if len(raw.Entries) > 0 {
		items := make([]VideoInfo, 0, len(raw.Entries))
		for _, e := range raw.Entries {
			item := VideoInfo{}
			if e.Title != nil {
				item.Title = *e.Title
			}
			if e.Duration != nil {
				item.Duration = *e.Duration
			}
			switch {
			case e.WebpageURL != nil:
				item.URL = *e.WebpageURL
			case e.URL != nil && strings.HasPrefix(*e.URL, "http"):
				item.URL = *e.URL
			case e.ID != "":
				item.URL = "https://www.youtube.com/watch?v=" + e.ID
			default:
				item.URL = rawURL
			}
			items = append(items, item)
		}
		return items, nil
	}

	// Single video.
	item := VideoInfo{URL: rawURL}
	if raw.Title != nil {
		item.Title = *raw.Title
	}
	if raw.Duration != nil {
		item.Duration = *raw.Duration
	}
	if raw.WebpageURL != nil {
		item.URL = *raw.WebpageURL
	}
	return []VideoInfo{item}, nil
}

var installOnce sync.Once

// ytdlpInstallTimeout is the maximum time allowed for the yt-dlp binary installation.
// 5 minutes is generous to accommodate slow networks and CI environments.
const ytdlpInstallTimeout = 5 * time.Minute

type DownloadProgress struct {
	Name          string
	Format        string
	FilePath      string
	Percent       int
	Completed     bool
	PlaylistIndex int // 0 for non-playlist; ≥1 for playlist tracks
	PlaylistCount int // total tracks in playlist, 0 if unknown
}

// IsPlaylistURL reports whether rawURL refers to a YouTube playlist
// (either a /playlist page or a watch URL that includes a list= parameter).
func IsPlaylistURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Query().Get("list") != "" || strings.Contains(u.Path, "/playlist")
}

func DownloadAudio(url string, ext string, tr *shared.TimeRange, cfg *config.Config) error {
	_, err := DownloadAudioWithProgress(context.TODO(), url, ext, tr, cfg, nil)
	return err
}

func DownloadAudioWithProgress(
	ctx context.Context,
	rawURL string,
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

	if IsPlaylistURL(rawURL) {
		dl.YesPlaylist()
	} else {
		dl.NoPlaylist()
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
			if update.Info != nil {
				if update.Info.Title != nil {
					progress.Name = *update.Info.Title
				}
				if update.Info.PlaylistIndex != nil {
					progress.PlaylistIndex = *update.Info.PlaylistIndex
				}
				if update.Info.PlaylistCount != nil {
					progress.PlaylistCount = *update.Info.PlaylistCount
				}
			}
			onProgress(progress)
		})
	}

	proc, err := dl.Run(ctx, rawURL)
	if err != nil {
		if proc != nil {
			return proc, fmt.Errorf("%s - %s", err, proc.Stderr)
		}
		return nil, err
	}

	return proc, nil
}
