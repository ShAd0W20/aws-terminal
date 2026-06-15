package ecs

import "github.com/charmbracelet/bubbles/key"

var (
	ecsUpKey            = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	ecsDownKey          = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	ecsEnterKey         = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/detail"))
	ecsBackKey          = key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b/esc", "back"))
	ecsRefreshKey       = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	ecsSearchKey        = key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "search"))
	ecsPagePrevKey      = key.NewBinding(key.WithKeys("left", "h", "pgup"), key.WithHelp("←/h", "prev page"))
	ecsPageNextKey      = key.NewBinding(key.WithKeys("right", "l", "pgdown"), key.WithHelp("→/l", "next page"))
	ecsPrevTabKey       = key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev tab"))
	ecsNextTabKey       = key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next tab"))
	ecsPrevContainerKey = key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "prev container"))
	ecsNextContainerKey = key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "next container"))
	ecsTabHelpKey       = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "sidebar"))
)
