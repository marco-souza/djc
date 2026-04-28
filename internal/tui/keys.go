package tui

import (
"github.com/charmbracelet/bubbles/key"
)

// keyMap defines all key bindings and implements help.KeyMap so the bubbles
// help component can render them automatically.
type keyMap struct {
	Move        key.Binding
	Ends        key.Binding
	Add         key.Binding
	Delete      key.Binding
	Export      key.Binding
	Config      key.Binding
	Refresh     key.Binding
	Play        key.Binding
	SeekBack    key.Binding
	SeekForward key.Binding
	VolumeDown  key.Binding
	VolumeUp    key.Binding
	Quit        key.Binding
	DropDB      key.Binding
}

// defaultKeys is the singleton key map used throughout the TUI.
var defaultKeys = keyMap{
Move: key.NewBinding(
key.WithKeys("j", "k", "up", "down"),
key.WithHelp("j/k", "move"),
),
Ends: key.NewBinding(
key.WithKeys("g", "G"),
key.WithHelp("g/G", "top/bottom"),
),
Add: key.NewBinding(
key.WithKeys("a"),
key.WithHelp("a", "add"),
),
Delete: key.NewBinding(
key.WithKeys("d"),
key.WithHelp("dd", "delete"),
),
Export: key.NewBinding(
key.WithKeys("e"),
key.WithHelp("e", "→mp3"),
),
Config: key.NewBinding(
key.WithKeys("c"),
key.WithHelp("c", "config"),
),
Refresh: key.NewBinding(
key.WithKeys("r"),
key.WithHelp("r", "refresh"),
),
Play: key.NewBinding(
key.WithKeys(" ", "f8"),
key.WithHelp("spc/F8", "play/pause"),
),
SeekBack: key.NewBinding(
key.WithKeys("[", "f7"),
key.WithHelp("[", "-10s"),
),
SeekForward: key.NewBinding(
key.WithKeys("]", "f9"),
key.WithHelp("]", "+10s"),
),
VolumeDown: key.NewBinding(
key.WithKeys("-"),
key.WithHelp("-", "vol-"),
),
VolumeUp: key.NewBinding(
key.WithKeys("="),
key.WithHelp("=", "vol+"),
),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	DropDB: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("^d", "drop db"),
	),
}

// ShortHelp returns the compact one-line help bar bindings.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Move, k.Ends, k.Add, k.Delete, k.Export, k.Config, k.Refresh, k.Play, k.SeekBack, k.SeekForward, k.VolumeDown, k.VolumeUp, k.Quit, k.DropDB}
}

// FullHelp returns the expanded multi-row help view bindings.
func (k keyMap) FullHelp() [][]key.Binding {
return [][]key.Binding{k.ShortHelp()}
}
