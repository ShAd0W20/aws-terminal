package shell

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"aws-terminal/internal/config"
)

func (m Model) shouldCheckUpdatesOnStart() bool {
	if m.updateService == nil || !config.CheckForUpdatesOnStart(m.preferences) {
		return false
	}
	version := strings.TrimSpace(strings.ToLower(m.updateService.CurrentVersion()))
	return version != "" && version != "dev" && version != "(devel)"
}

func (m *Model) checkUpdatesCmd(startup bool) tea.Cmd {
	if m.updateService == nil {
		return nil
	}
	m.updateCheckBusy = true
	return func() tea.Msg {
		timeout := 8 * time.Second
		if startup {
			timeout = 2 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, err := m.updateService.Check(ctx)
		return updateCheckedMsg{result: result, startup: startup, err: err}
	}
}

func (m *Model) installUpdateCmd() tea.Cmd {
	if m.updateService == nil {
		return nil
	}
	m.updateInstallBusy = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		result, err := m.updateService.InstallLatest(ctx)
		return updateInstalledMsg{result: result, err: err}
	}
}

func (m *Model) disableStartupUpdateChecks() tea.Cmd {
	m.preferences = config.WithCheckForUpdatesOnStart(m.preferences, false)
	if m.preferenceStore != nil {
		_ = m.preferenceStore.Save(m.preferences)
	}
	m.statusMessage = "Startup update checks disabled. Manual checks remain available from commands."
	m.errorMessage = ""
	return nil
}
