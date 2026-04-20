package tui

import (
"context"
"os/exec"
"time"

"marco-souza/djc/internal/config"
"marco-souza/djc/internal/library"
"marco-souza/djc/internal/youtube"

"github.com/charmbracelet/bubbles/help"
"github.com/charmbracelet/bubbles/progress"
"github.com/charmbracelet/bubbles/spinner"
"github.com/charmbracelet/bubbles/textinput"
tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"
)

// ── model ───────────────────────────────────────────────────────────────────

type Model struct {
repo *library.Repository
cfg  *config.Config

songs  []library.Song
cursor int // active row index
offset int // first rendered row index (scroll)
mode   Mode

// vim-style dd
pendingD bool

// add modal
addInput textinput.Model

// confirm modal
confirmURL    string
confirmItems  []youtube.VideoInfo // nil = still loading
confirmSel    []bool
confirmCursor int
confirmOffset int

// download queue (processed one at a time)
downloadQueue []queuedDownload

// delete confirmation: true = "Delete" button selected, false = "Cancel"
deleteConf bool

// cancels for in-progress downloads
cancels map[int64]context.CancelFunc

// playback
playerProc    *exec.Cmd
playerSongID  int64 // 0 = nothing playing
playerPaused  bool
playerElapsed time.Duration // approximate elapsed time (updated by ticker)
playerDuration time.Duration // 0 = unknown (queried via mpv IPC)
playerVolume  int    // 0-100, default 100
playerIPC     string // mpv IPC socket path, empty for non-mpv players

// config modal
configInputs [4]textinput.Model
configFocus  int

// UI components
spinner          spinner.Model
helpModel        help.Model
downloadProgress progress.Model
playerProgress   progress.Model // progress bar for the player bar

// terminal dimensions
width, height int
ready         bool

// status bar
statusMsg   string
statusError bool
}

// ── constructor ─────────────────────────────────────────────────────────────

func NewModel(repo *library.Repository, cfg *config.Config) Model {
inp := textinput.New()
inp.Placeholder = "https://www.youtube.com/watch?v=..."
inp.CharLimit = 1024
inp.PromptStyle = lipgloss.NewStyle().Foreground(clrAccent)
inp.TextStyle = lipgloss.NewStyle().Foreground(clrBright)

sp := spinner.New(
spinner.WithSpinner(spinner.MiniDot),
spinner.WithStyle(lipgloss.NewStyle().Foreground(clrYellow)),
)

h := help.New()
h.ShortSeparator = "  │  "
h.Styles.ShortKey = sKey
h.Styles.ShortDesc = sMuted
h.Styles.ShortSeparator = sMuted
h.Styles.Ellipsis = sMuted

pb := progress.New(
progress.WithWidth(10),
progress.WithoutPercentage(),
progress.WithGradient(string(clrYellow), string(clrGreen)),
)

playerPb := progress.New(
progress.WithWidth(16),
progress.WithoutPercentage(),
progress.WithGradient(string(clrBlue), string(clrAccent)),
)

return Model{
repo:             repo,
cfg:              cfg,
addInput:         inp,
spinner:          sp,
helpModel:        h,
downloadProgress: pb,
playerProgress:   playerPb,
playerVolume:     100,
cancels:          map[int64]context.CancelFunc{},
configInputs:     newConfigInputs(),
}
}

// ── init ────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
return tea.Batch(refreshSongsCmd(m.repo), m.spinner.Tick)
}
