package ec2

import "github.com/charmbracelet/bubbles/key"

var (
	ec2UpKey        = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	ec2DownKey      = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	ec2EnterKey     = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/confirm"))
	ec2BackKey      = key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back"))
	ec2CancelKey    = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel/clear"))
	ec2RefreshKey   = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	ec2SearchKey    = key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "search"))
	ec2ConnectKey   = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect"))
	ec2StopKey      = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stop"))
	ec2TerminateKey = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "terminate"))
	ec2TabKey       = key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab/shift+tab", "switch focus"))
)
