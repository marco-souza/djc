package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// ── colour palette ──────────────────────────────────────────────────────────

const (
	clrAccent   = lipgloss.Color("205") // hot-pink
	clrBlue     = lipgloss.Color("69")  // cornflower
	clrGreen    = lipgloss.Color("42")  // emerald
	clrRed      = lipgloss.Color("196") // red
	clrYellow   = lipgloss.Color("220") // amber
	clrMuted    = lipgloss.Color("240") // dark-grey
	clrSubtle   = lipgloss.Color("250") // light-grey
	clrBright   = lipgloss.Color("255") // white
	clrHiliteBg = lipgloss.Color("62")  // blue selection bg
	clrRedBg    = lipgloss.Color("88")  // dark-red for delete confirm
)

// ── styles ──────────────────────────────────────────────────────────────────

var (
	sTitle = lipgloss.NewStyle().Bold(true).Foreground(clrAccent)

	sSep = lipgloss.NewStyle().Foreground(clrMuted)

	sColHdr = lipgloss.NewStyle().Bold(true).Foreground(clrBlue)

	sSel = lipgloss.NewStyle().
		Foreground(clrBright).
		Background(clrHiliteBg).
		Bold(true)

	sNormal = lipgloss.NewStyle().Foreground(clrSubtle)
	sDling  = lipgloss.NewStyle().Foreground(clrYellow)
	sDone   = lipgloss.NewStyle().Foreground(clrGreen)
	sFailed = lipgloss.NewStyle().Foreground(clrRed)
	sMuted  = lipgloss.NewStyle().Foreground(clrMuted)
	sBright = lipgloss.NewStyle().Foreground(clrBright).Bold(true)
	sGreen  = lipgloss.NewStyle().Foreground(clrGreen)
	sRed    = lipgloss.NewStyle().Foreground(clrRed)
	sKey    = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)

	sDLabelSt = lipgloss.NewStyle().Bold(true).Foreground(clrBlue)
	sDValueSt = lipgloss.NewStyle().Foreground(clrSubtle)

	sModal = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(clrAccent).
		Padding(1, 3)

	sDelModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(clrRed).
			Padding(1, 3)

	sCfgModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(clrBlue).
			Padding(1, 3)

	sBtnActive = lipgloss.NewStyle().Bold(true).Padding(0, 2)
	sBtnNormal = lipgloss.NewStyle().Foreground(clrMuted).Padding(0, 2)

	// Player bar — active track name / state icon
	sPlayer = lipgloss.NewStyle().Bold(true).Foreground(clrAccent)
)

// ── config field metadata ────────────────────────────────────────────────────

var configLabels = [4]string{
	"Download Directory",
	"Audio Format",
	"Audio Quality",
	"Output Template",
}

func newConfigInputs() [4]textinput.Model {
	var inputs [4]textinput.Model
	for i := range inputs {
		inp := textinput.New()
		inp.Placeholder = configLabels[i]
		inp.CharLimit = 512
		inp.PromptStyle = lipgloss.NewStyle().Foreground(clrAccent)
		inp.TextStyle = lipgloss.NewStyle().Foreground(clrBright)
		inputs[i] = inp
	}
	return inputs
}

// ── fixed column widths (name is dynamic) ───────────────────────────────────

const (
	colFmt       = 6
	colStatus    = 22
	colDate      = 12
	maxNameWidth = 30
	// row = cursor(2) + name(dynamic) + sp(1) + fmt + sp(1) + status + sp(1) + date
	fixedWidth = 2 + 1 + colFmt + 1 + colStatus + 1 + colDate // = 45
	detailRows = 5                                             // content lines in the details panel
)
