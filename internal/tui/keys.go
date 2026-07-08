package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the dashboard.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	GoTop      key.Binding
	GoBot      key.Binding
	Search     key.Binding
	Sort       key.Binding
	OnlineOnly key.Binding
	Block      key.Binding
	Unblock    key.Binding
	Reload     key.Binding
	Follow     key.Binding
	Help       key.Binding
	Quit       key.Binding
	Enter      key.Binding
	Cancel     key.Binding
	NextTab    key.Binding
	PrevTab    key.Binding

	// rules view
	Rules       key.Binding
	RuleEnable  key.Binding
	RuleDisable key.Binding
	RuleDelete  key.Binding

	// alarms view
	Alarms       key.Binding
	AlarmArchive key.Binding

	// networks view
	Networks key.Binding
	WAN      key.Binding
	Data     key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		GoTop:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		GoBot:      key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Sort:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		OnlineOnly: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "online only")),
		Block:      key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "block")),
		Unblock:    key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unblock")),
		Reload:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		Follow:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "live")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		NextTab:    key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab/→", "next view")),
		PrevTab:    key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("⇧tab/←", "prev view")),

		Rules:       key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rules")),
		RuleEnable:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "enable")),
		RuleDisable: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "disable")),
		RuleDelete:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),

		Alarms:       key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "alarms")),
		AlarmArchive: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),

		Networks: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "networks")),
		WAN:      key.NewBinding(key.WithKeys("W"), key.WithHelp("W", "wan")),
		Data:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "data")),
	}
}

// DataHelp is the one-line footer help shown in the data-usage view.
func (k KeyMap) DataHelp() string {
	return "tab views • r reload • esc devices • q quit"
}

// NetworksHelp is the one-line footer help shown in the networks view.
func (k KeyMap) NetworksHelp() string {
	return "↑↓ nav • enter detail • / search • s sort • tab views • r reload • esc devices • q quit"
}

// WANHelp is the one-line footer help shown in the WAN view.
func (k KeyMap) WANHelp() string {
	return "↑↓ nav • enter detail • / search • s sort • tab views • r reload • esc devices • q quit"
}

// RulesHelp is the one-line footer help shown in the rules view.
func (k KeyMap) RulesHelp() string {
	return "↑↓ nav • enter detail • / search • s sort • e/d/x en/dis/del • tab views • r reload • esc devices • q quit"
}

// AlarmsHelp is the one-line footer help shown in the alarms view.
func (k KeyMap) AlarmsHelp() string {
	return "↑↓ nav • enter detail • / search • s sort • a archive • x delete • tab views • r reload • esc devices • q quit"
}

// ShortHelp is the one-line footer help for normal mode.
func (k KeyMap) ShortHelp() string {
	return "↑↓ nav • tab/→ views • enter detail • / search • s sort • f live • o online • b block • u unblock • ? help • q quit"
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
		{"tab / ⇧tab", "Next / previous view"},
		{"h/l, ←/→", "Previous / next view"},
		{"1–6", "Jump to a view (devices…data)"},
		{"enter", "Open detail for the selected item"},
		{"/", "Search / filter the current list"},
		{"s", "Cycle sort (default ↔ by name)"},
		{"o", "Toggle online-only filter (devices)"},
		{"b", "Block selected device"},
		{"u", "Unblock selected device"},
		{"R/A/N/W/D", "Jump to rules/alarms/networks/wan/data"},
		{"e/d/x", "Enable/disable/delete (rules) • archive/delete (alarms)"},
		{"r", "Reload list"},
		{"f", "Toggle live auto-refresh"},
		{"?", "Toggle this help"},
		{"q / ctrl+c", "Quit"},
	}
}
