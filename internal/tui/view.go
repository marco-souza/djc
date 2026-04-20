package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"marco-souza/djc/internal/library"

	"github.com/charmbracelet/lipgloss"
)

// ── view ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return "Loading…\n"
	}
	switch m.mode {
	case modeAdd:
		return m.viewAdd()
	case modeConfirm:
		return m.viewConfirm()
	case modeDelete:
		return m.viewDelete()
	case modeConfig:
		return m.viewConfig()
	default:
		return m.viewMain()
	}
}

// viewMain renders the full-page list + details split.
//
// Layout (each line):
//
//	1        title bar
//	2        player bar (always visible; shows "Nothing playing" when idle)
//	3        ─── separator
//	4        table column headers
//	5..5+lh  song rows  (lh = listHeight())
//	+1       ─── separator
//	+5       detail panel  (detailRows content lines)
//	+1       ─── separator
//	+1       status line
//	+1       help bar
func (m Model) viewMain() string {
	w := m.width

	sep := sSep.Render(strings.Repeat("─", w))

	// 1. Title
	title := sTitle.Render("🎧  DJ Companion")

	// 2. Player bar
	playerBar := m.renderPlayerBar(w)

	// 3. Table header
	header := m.renderHeader(w)

	// 4. Song list rows (exactly listHeight lines)
	list := m.renderList(w)

	// 5. Detail panel
	details := m.renderDetails(w)

	// 6. Status
	status := m.renderStatus(w)

	// 7. Help bar
	help := m.renderHelp()

	parts := []string{
		title,
		playerBar,
		sep,
		header,
		list,
		sep,
		details,
		sep,
		status,
		help,
	}
	return strings.Join(parts, "\n")
}

// viewAdd renders the full screen with a centered "Add Song" modal.
func (m Model) viewAdd() string {
	title := sTitle.Render("  Add New Song")
	prompt := sMuted.Render("  YouTube URL")
	input := "  " + m.addInput.View()
	hint := sMuted.Render("  enter  confirm  •  esc  cancel")

	content := strings.Join([]string{title, "", prompt, input, "", hint}, "\n")
	box := sModal.Width(max(60, m.width/2)).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(clrMuted),
	)
}

// viewConfirm renders the confirmation / playlist-select modal.
// While metadata is loading (confirmItems == nil) it shows a spinner.
// For a single video it shows a simple confirm prompt.
// For playlists it shows a scrollable multi-select checklist.
func (m Model) viewConfirm() string {
	modalW := max(64, m.width*2/3)
	innerW := modalW - 8 // subtract border(2) + padding(3+3)

	var lines []string

	if m.confirmItems == nil {
		// ── loading ───────────────────────────────────────────────────────
		lines = []string{
			sTitle.Render("  Fetching info…"),
			"",
			"  " + m.spinner.View() + "  Please wait…",
			"",
		}
	} else if len(m.confirmItems) == 1 {
		// ── single video confirm ──────────────────────────────────────────
		item := m.confirmItems[0]
		name := item.Title
		if name == "" {
			name = "(title unknown)"
		}
		lines = []string{
			sTitle.Render("  Download?"),
			"",
			"  " + sBright.Render(truncate(name, innerW-4)),
		}
		if item.Duration > 0 {
			lines = append(lines, "  "+sMuted.Render("Duration: ")+
				formatDuration(time.Duration(item.Duration)*time.Second))
		}
		lines = append(lines,
			"",
			sMuted.Render("  enter: download  •  esc: back"),
		)
	} else {
		// ── playlist multi-select ─────────────────────────────────────────
		selectedCount := 0
		for _, s := range m.confirmSel {
			if s {
				selectedCount++
			}
		}

		lines = append(lines,
			sTitle.Render(fmt.Sprintf("  Select songs  (%d tracks)", len(m.confirmItems))),
			"",
		)

		end := m.confirmOffset + confirmModalMaxItems
		if end > len(m.confirmItems) {
			end = len(m.confirmItems)
		}
		for i := m.confirmOffset; i < end; i++ {
			item := m.confirmItems[i]

			cursorStr := "  "
			if i == m.confirmCursor {
				cursorStr = "▶ "
			}
			check := "[ ]"
			if i < len(m.confirmSel) && m.confirmSel[i] {
				check = "[x]"
			}
			title := item.Title
			if title == "" {
				title = "(untitled)"
			}
			dur := ""
			if item.Duration > 0 {
				dur = "  " + formatDuration(time.Duration(item.Duration)*time.Second)
			}
			row := cursorStr + check + " " + truncate(title, innerW-10) + dur

			switch {
			case i == m.confirmCursor:
				lines = append(lines, sSel.Render(row))
			case i < len(m.confirmSel) && m.confirmSel[i]:
				lines = append(lines, sGreen.Render(row))
			default:
				lines = append(lines, sMuted.Render(row))
			}
		}

		// Scroll indicator when list is longer than the visible window.
		if len(m.confirmItems) > confirmModalMaxItems {
			pct := 0
			if len(m.confirmItems) > 1 {
				pct = (m.confirmCursor * 100) / (len(m.confirmItems) - 1)
			}
			lines = append(lines, sMuted.Render(fmt.Sprintf("  ── %d%% ──", pct)))
		}

		lines = append(lines,
			"",
			sMuted.Render(fmt.Sprintf("  %d of %d selected", selectedCount, len(m.confirmItems))),
			"",
			sMuted.Render("  j/k: navigate  spc: toggle  a: all  n: none  enter: download  esc: back"),
		)
	}

	content := strings.Join(lines, "\n")
	box := sModal.Width(modalW).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(clrMuted),
	)
}
func (m Model) viewDelete() string {
	songName := "unknown"
	if len(m.songs) > 0 {
		songName = m.songs[m.cursor].Name
	}
	name := truncate(songName, 42)

	title := sRed.Bold(true).Render("  Delete Song?")
	nameStr := lipgloss.NewStyle().Italic(true).Foreground(clrSubtle).Render(`  "` + name + `"`)
	warning := sMuted.Render("  This will also remove the file from disk.")

	var btnDel, btnCancel string
	if m.deleteConf {
		btnDel = sBtnActive.Copy().Background(clrRedBg).Foreground(clrBright).Render("  Delete  ")
		btnCancel = sBtnNormal.Render("  Cancel  ")
	} else {
		btnDel = sBtnNormal.Render("  Delete  ")
		btnCancel = sBtnActive.Copy().Background(clrHiliteBg).Foreground(clrBright).Render("  Cancel  ")
	}
	buttons := "  " + btnDel + "   " + btnCancel

	hint := sMuted.Render("  h/l  toggle  •  enter  confirm  •  esc / n  cancel")
	yHint := sMuted.Render("  y  delete immediately")

	content := strings.Join([]string{title, "", nameStr, warning, "", buttons, "", hint, yHint}, "\n")
	box := sDelModal.Width(max(60, m.width/2)).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(clrMuted),
	)
}

// viewConfig renders a centred config editor modal.
func (m Model) viewConfig() string {
	title := sTitle.Render("⚙  Configuration")

	var rows []string
	rows = append(rows, title, "")
	for i, inp := range m.configInputs {
		if i == m.configFocus {
			rows = append(rows, "  "+sDLabelSt.Render(configLabels[i]))
		} else {
			rows = append(rows, "  "+sMuted.Render(configLabels[i]))
		}
		rows = append(rows, "  "+inp.View(), "")
	}

	hint := sMuted.Render("  tab/j/k  navigate  •  ctrl+s / enter  save  •  esc  cancel")
	rows = append(rows, hint)

	content := strings.Join(rows, "\n")
	box := sCfgModal.Width(max(60, m.width*2/3)).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(clrMuted),
	)
}

// ── rendering helpers ───────────────────────────────────────────────────────

// renderPlayerBar renders the persistent 1-line music player bar.
// When nothing is playing it shows a hint; when a track is active it shows
// the song name, play/pause state, seek controls, a progress bar, elapsed/total
// time, and volume level.
func (m Model) renderPlayerBar(w int) string {
	if m.playerSongID == 0 {
		hint := "  ♫  Nothing playing  ·  spc: play  ·  [/]: seek ±10s  ·  -/=: vol"
		return sMuted.Width(w).Render(hint)
	}

	playIcon := "▶"
	if m.playerPaused {
		playIcon = "⏸"
	}

	// Time string
	elapsed := formatDuration(m.playerElapsed)
	timeStr := elapsed
	if m.playerDuration > 0 {
		timeStr = elapsed + "/" + formatDuration(m.playerDuration)
	}

	// Right section: controls + time + volume (compute width first so the name
	// section can fill exactly the remaining space).
	rightStr := fmt.Sprintf("  -10 %s +10  %s  vol:%d  ", playIcon, timeStr, m.playerVolume)
	rightW := lipgloss.Width(rightStr)

	// Progress bar (scales with terminal width, minimum 6 chars).
	var pct float64
	if m.playerDuration > 0 {
		pct = float64(m.playerElapsed) / float64(m.playerDuration)
		if pct > 1 {
			pct = 1
		}
	}
	pbW := max(6, w/6)
	pb := m.playerProgress // copy — don't mutate the model field
	pb.Width = pbW
	bar := pb.ViewAs(pct)
	barW := lipgloss.Width(bar)

	// Song name fills the remaining width.
	// prefixW accounts for the two leading spaces + state icon + trailing space: "  ▶ "
	const prefixW = 4
	nameW := w - rightW - barW - prefixW
	if nameW < 2 {
		nameW = 2
	}
	var songName string
	for _, s := range m.songs {
		if s.ID == m.playerSongID {
			songName = s.Name
			break
		}
	}
	nameSection := sPlayer.Render(fmt.Sprintf("  %s %s", playIcon, truncate(songName, nameW)))
	// Pad to align the progress bar flush with the right section.
	if pad := prefixW + nameW - lipgloss.Width(nameSection); pad > 0 {
		nameSection += strings.Repeat(" ", pad)
	}

	return nameSection + bar + sMuted.Render(rightStr)
}

// formatDuration formats a duration as m:ss (e.g. "3:05").
func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func (m Model) nameWidth() int {
	n := m.width - fixedWidth
	if n < 8 {
		return 8
	}
	if n > maxNameWidth {
		return maxNameWidth
	}
	return n
}

func (m Model) renderHeader(w int) string {
	nw := m.nameWidth()
	h := fmt.Sprintf("  %-*s %-*s %-*s %-*s",
		nw, "SONG",
		colFmt, "FMT",
		colStatus, "STATUS",
		colDate, "DATE",
	)
	return sColHdr.Render(truncatePlain(h, w))
}

func (m Model) renderList(w int) string {
	lh := m.listHeight()

	if len(m.songs) == 0 {
		emptyMsg := sMuted.Render("  No songs yet — press a to add one")
		lines := make([]string, lh)
		lines[0] = emptyMsg
		for i := 1; i < lh; i++ {
			lines[i] = ""
		}
		return strings.Join(lines, "\n")
	}

	rows := make([]string, lh)
	for i := 0; i < lh; i++ {
		idx := m.offset + i
		if idx < len(m.songs) {
			rows[i] = m.renderRow(idx, w)
		} else {
			rows[i] = ""
		}
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderRow(idx, w int) string {
	song := m.songs[idx]
	selected := idx == m.cursor
	nw := m.nameWidth()

	cursor := "  "
	if selected {
		cursor = "▶ "
	}

	name := truncate(song.Name, nw)
	fmtStr := truncate(strings.ToLower(song.Format), colFmt)
	statusStr := m.rowStatus(song, colStatus)
	dateStr := song.CreatedAt.Local().Format("Jan 02 15:04")

	line := fmt.Sprintf("%s%-*s %-*s %-*s %-*s",
		cursor,
		nw, name,
		colFmt, fmtStr,
		colStatus, statusStr,
		colDate, dateStr,
	)
	line = truncatePlain(line, w)

	if selected {
		return sSel.Width(w).Render(line)
	}
	switch {
	case strings.HasPrefix(song.Status, "failed"):
		return sFailed.Width(w).Render(line)
	case song.Status == "queued":
		return sQueued.Width(w).Render(line)
	case song.Status == "downloading":
		return sDling.Width(w).Render(line)
	default:
		return sNormal.Width(w).Render(line)
	}
}

func (m Model) rowStatus(song library.Song, width int) string {
	// Show playback state for the currently playing/paused song.
	if m.playerSongID == song.ID {
		if m.playerPaused {
			return truncate("⏸ paused", width)
		}
		return truncate("▶ playing", width)
	}
	switch {
	case song.Status == "queued":
		return truncate("⧗ in queue", width)
	case song.Status == "downloading":
		bar := m.downloadProgress.ViewAs(float64(song.Progress) / 100)
		pct := fmt.Sprintf("%3d%%", song.Progress)
		s := m.spinner.View() + " " + bar + " " + pct
		return truncate(s, width)
	case song.Status == "downloaded":
		return truncate("✓ downloaded", width)
	case strings.HasPrefix(song.Status, "failed"):
		return truncate("✗ "+strings.TrimPrefix(song.Status, "failed: "), width)
	default:
		return truncate(song.Status, width)
	}
}

func (m Model) renderDetails(w int) string {
	pad := lipgloss.NewStyle().Width(w)

	if len(m.songs) == 0 {
		lines := make([]string, detailRows)
		lines[0] = pad.Render(sMuted.Render("  Select a song to see details"))
		for i := 1; i < detailRows; i++ {
			lines[i] = pad.Render("")
		}
		return strings.Join(lines, "\n")
	}

	song := m.songs[m.cursor]
	nw := w - 12

	label := func(s string) string {
		return sDLabelSt.Width(8).Render(s + ":")
	}
	val := func(s string) string {
		return sDValueSt.Render(s)
	}

	titleLine := pad.Render("  " + sBright.Render(truncate(song.Name, w-4)))
	fmtDate := pad.Render("  " + label("Format") + val(strings.ToUpper(song.Format)) +
		"   " + label("Added") + val(song.CreatedAt.Local().Format("Mon Jan 02, 2006 15:04")))
	statusLine := pad.Render("  " + label("Status") + val(formatSongStatus(song)))
	pathLine := pad.Render("  " + label("Path") + val(truncate(song.FilePath, nw)))
	srcLine := pad.Render("  " + label("Source") + val(truncate(song.SourceURL, nw)))

	return strings.Join([]string{titleLine, fmtDate, statusLine, pathLine, srcLine}, "\n")
}

func (m Model) renderStatus(w int) string {
	pad := lipgloss.NewStyle().Width(w)
	if m.statusMsg == "" {
		return pad.Render("")
	}
	if m.statusError {
		return pad.Render(sRed.Render(" " + m.statusMsg))
	}
	return pad.Render(sGreen.Render(" " + m.statusMsg))
}

func (m Model) renderHelp() string {
	return " " + m.helpModel.View(defaultKeys)
}

// ── pure helpers ─────────────────────────────────────────────────────────────

func formatSongStatus(song library.Song) string {
	switch {
	case song.Status == "queued":
		return "In Queue"
	case song.Status == "downloading":
		return fmt.Sprintf("Downloading… %d%%", song.Progress)
	case song.Status == "downloaded":
		return "Downloaded"
	default:
		return song.Status
	}
}

// truncate shortens s to at most limit runes, appending "…" if trimmed.
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

// truncatePlain truncates a plain (possibly ANSI-free) string by rune count.
// Used only for lines where we know there are no escape codes.
func truncatePlain(s string, limit int) string {
	if len([]rune(s)) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit])
}

func pick(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func extensionOr(defaultValue, filePath string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
	if ext != "" {
		return ext
	}
	return defaultValue
}
