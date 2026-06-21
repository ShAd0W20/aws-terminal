package pages

import (
	"aws-terminal/internal/ui/pageapi"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	domainauth "aws-terminal/internal/domain/auth"
	"aws-terminal/internal/ui/styles"
)

type DashboardPage struct{}

func NewDashboardPage() *DashboardPage {
	return &DashboardPage{}
}

func (*DashboardPage) ID() string {
	return "dashboard"
}

func (*DashboardPage) Title() string {
	return "Dashboard"
}

func (*DashboardPage) Description() string {
	return "Overview, AWS context, and quick actions."
}

func (*DashboardPage) OnStateChanged(pageapi.State) tea.Cmd {
	return nil
}

func (*DashboardPage) SetFocused(bool) tea.Cmd {
	return nil
}

func (*DashboardPage) Update(tea.Msg, pageapi.State) tea.Cmd {
	return nil
}

func (*DashboardPage) ShortHelp() []key.Binding {
	return nil
}

func (*DashboardPage) FullHelp() [][]key.Binding {
	return nil
}

func (*DashboardPage) View(state pageapi.State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := []string{
		styles.SectionTitleStyle.Render("Dashboard"),
		styles.SubtitleStyle.Render("Current AWS context and next step"),
		"",
	}

	if state.ActiveSession == nil {
		lines = append(lines,
			styles.MutedStyle.Render("No active AWS session"),
		)
	} else {
		lines = append(lines,
			styles.StatusStyle.Render("Active profile: "+state.ActiveSession.Profile),
			fmt.Sprintf("Account: %s", valueOrFallback(state.ActiveSession.Account, "unknown")),
		)
	}

	region := strings.TrimSpace(state.SelectedRegion)
	if region == "" && state.ActiveSession != nil {
		region = strings.TrimSpace(state.ActiveSession.Region)
	}
	if region != "" {
		lines = append(lines, fmt.Sprintf("Region: %s", region))
	}

	if state.HighlightedProfile != nil {
		lines = append(lines, fmt.Sprintf("Selected profile: %s", state.HighlightedProfile.Name))
	}

	if state.ProfileBusy {
		lines = append(lines, "", styles.StatusStyle.Render("Authentication in progress..."))
	}
	if state.StatusMessage != "" {
		lines = append(lines, "", styles.StatusStyle.Render(state.StatusMessage))
	}
	if state.ErrorMessage != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(state.ErrorMessage))
	}

	if state.AuthPrompt != nil {
		lines = append(lines,
			"",
			styles.MutedStyle.Render("SSO sign-in"),
			fmt.Sprintf("Profile: %s", state.AuthPrompt.ProfileName),
			fmt.Sprintf("Code: %s", state.AuthPrompt.UserCode),
			fmt.Sprintf("URL: %s", preferredVerificationURL(*state.AuthPrompt)),
		)
		if state.AuthPrompt.BrowserOpened {
			lines = append(lines, styles.StatusStyle.Render("Browser opened automatically."))
		}
		if state.AuthPrompt.BrowserOpenError != "" {
			lines = append(lines, styles.MutedStyle.Render("Browser: "+state.AuthPrompt.BrowserOpenError))
		}
	}

	if state.PageFocused {
		lines = append(lines, "", styles.MutedStyle.Render("Page focus active • Tab or Shift+Tab returns to the sidebar"))
	}

	lines = append(lines, "", styles.MutedStyle.Render("Next"))
	if state.ActiveSession == nil {
		lines = append(lines,
			"• Pick a region",
			"• Authenticate a profile",
			"• Open the S3 page to start syncing",
		)
	} else {
		lines = append(lines,
			"• Open the S3 page",
			"• Choose a bucket",
			"• Select a local file or folder to sync",
		)
	}

	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}

func preferredVerificationURL(prompt domainauth.Prompt) string {
	if strings.TrimSpace(prompt.VerificationURIComplete) != "" {
		return prompt.VerificationURIComplete
	}

	return prompt.VerificationURI
}
