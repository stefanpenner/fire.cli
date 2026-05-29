package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the dashboard.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	GoTop   key.Binding
	GoBot   key.Binding
	Search  key.Binding
	Block   key.Binding
	Unblock key.Binding
	Reload  key.Binding
	Help    key.Binding
	Quit    key.Binding
	Enter   key.Binding
	Cancel  key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		GoTop:   key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		GoBot:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		Search:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Block:   key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "block")),
		Unblock: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unblock")),
		Reload:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// ShortHelp is the one-line footer help for normal mode.
func (k KeyMap) ShortHelp() string {
	return "↑↓ nav • enter detail • / search • b block • u unblock • r reload • ? help • q quit"
}

// SearchHelp is the footer help shown while typing a search.
func (k KeyMap) SearchHelp() string {
	return "type to filter • enter accept • esc cancel"
}

// FullHelp returns all bindings for the help modal as label/desc rows.
func (k KeyMap) FullHelp() [][2]string {
	return [][2]string{
		{"↑/k, ↓/j", "Move selection"},
		{"g / G", "Jump to top / bottom"},
		{"enter", "Device detail (traffic)"},
		{"/", "Search devices (name, IP, MAC)"},
		{"b", "Block selected device"},
		{"u", "Unblock selected device"},
		{"r", "Reload device list"},
		{"?", "Toggle this help"},
		{"q / ctrl+c", "Quit"},
	}
}
