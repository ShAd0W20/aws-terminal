package sqs

import "github.com/charmbracelet/bubbles/key"

var (
	sqsUpKey      = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	sqsDownKey    = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	sqsEnterKey   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/confirm"))
	sqsBackKey    = key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back"))
	sqsRefreshKey = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	sqsSearchKey  = key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "search"))
	sqsPullKey    = key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pull messages"))
	sqsPurgeKey   = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "purge"))
	sqsCancelKey  = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel/clear"))
	sqsTabKey     = key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab/shift+tab", "switch focus"))
)
