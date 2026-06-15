package shell

import (
	"context"
	"errors"
	"fmt"
	"time"

	domainprofile "aws-terminal/internal/domain/profile"
	"aws-terminal/internal/ui/pages"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.help.Width = max(0, m.innerWidth())
		m.help.ShowAll = m.width >= 110
		m.syncSidebarListsLayout()
		return m, m.currentPageUpdateCmd(msg)
	case spinner.TickMsg:
		if !m.profileBusy {
			return m, m.currentPageUpdateCmd(msg)
		}

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, tea.Batch(cmd, m.currentPageUpdateCmd(msg))
	case profilesLoadedMsg:
		m.loadProfilesCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return m, nil
		}
		if msg.err != nil {
			m.errorMessage = fmt.Sprintf("Failed to load AWS profiles: %v", msg.err)
			m.statusMessage = ""
			return m, m.currentPageStateCmd()
		}

		m.profiles = msg.profiles
		m.syncProfileSelection()
		m.syncRegionSelection()
		m.errorMessage = ""
		if len(m.profiles) == 0 {
			m.statusMessage = "No AWS profiles found. Check ~/.aws/config and ~/.aws/credentials."
		} else {
			m.statusMessage = "Choose a region, authenticate a profile, then move into the active page workflow."
		}
		return m, m.currentPageStateCmd()
	case ssoSessionCheckedMsg:
		m.checkSSOSessionCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			m.profileBusy = false
			m.refreshProfileList("")
			return m, m.currentPageStateCmd()
		}
		if msg.reusable && msg.err == nil {
			m.rememberProfile(msg.profile.Name)
			m.statusMessage = fmt.Sprintf("Reusing cached AWS SSO session for profile %q. Loading caller identity in %s...", msg.profile.Name, m.activeRegion())
			m.errorMessage = ""
			return m, tea.Batch(m.activateProfileCmd(msg.profile.Name), m.currentPageStateCmd())
		}

		m.statusMessage = fmt.Sprintf("Starting native AWS SSO login for profile %q in region %q...", msg.profile.Name, m.activeRegion())
		m.errorMessage = ""
		return m, tea.Batch(m.startSSOLoginCmd(msg.profile), m.currentPageStateCmd())
	case ssoLoginStartedMsg:
		m.startSSOLoginCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			m.profileBusy = false
			m.refreshProfileList("")
			return m, m.currentPageStateCmd()
		}
		if msg.err != nil {
			m.profileBusy = false
			m.authPrompt = nil
			m.errorMessage = fmt.Sprintf("Unable to start native AWS SSO login: %v", msg.err)
			m.statusMessage = ""
			m.refreshProfileList("")
			return m, m.currentPageStateCmd()
		}

		m.authPrompt = &msg.prompt
		m.errorMessage = ""
		m.refreshProfileList(msg.prompt.ProfileName)
		if msg.prompt.BrowserOpened {
			m.statusMessage = fmt.Sprintf("Opened your browser for profile %q. Approve the sign-in, then the app will continue automatically.", msg.prompt.ProfileName)
		} else {
			m.statusMessage = fmt.Sprintf("Native AWS SSO login started for %q. Open the verification URL and enter the displayed code.", msg.prompt.ProfileName)
		}
		return m, tea.Batch(m.spinner.Tick, m.pollSSOLoginCmd(msg.prompt.SessionID, msg.prompt.PollInterval), m.currentPageStateCmd())
	case ssoLoginPolledMsg:
		m.pollSSOLoginCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			m.profileBusy = false
			m.refreshProfileList("")
			return m, m.currentPageStateCmd()
		}
		if msg.err != nil {
			m.profileBusy = false
			m.errorMessage = fmt.Sprintf("Native AWS SSO login failed: %v", msg.err)
			m.statusMessage = ""
			m.refreshProfileList("")
			return m, m.currentPageStateCmd()
		}

		if !msg.result.Done {
			if m.authPrompt != nil {
				m.statusMessage = fmt.Sprintf("Waiting for AWS SSO approval for %q...", m.authPrompt.ProfileName)
			}
			return m, tea.Batch(m.spinner.Tick, m.pollSSOLoginCmd(msg.sessionID, msg.result.NextPollInterval), m.currentPageStateCmd())
		}

		profileName := "selected profile"
		if m.authPrompt != nil && m.authPrompt.ProfileName != "" {
			profileName = m.authPrompt.ProfileName
		}
		m.statusMessage = fmt.Sprintf("Native AWS SSO login complete for %q. Loading caller identity in %s...", profileName, m.activeRegion())
		return m, tea.Batch(m.activateProfileCmd(profileName), m.currentPageStateCmd())
	case profileActivatedMsg:
		m.activateProfileCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			m.profileBusy = false
			m.refreshProfileList("")
			return m, m.currentPageStateCmd()
		}
		m.profileBusy = false
		if msg.err != nil {
			m.errorMessage = fmt.Sprintf("Unable to load credentials for the selected profile: %v", msg.err)
			m.statusMessage = ""
			m.refreshProfileList("")
			return m, m.currentPageStateCmd()
		}

		m.activeSession = &msg.session
		m.ensureRegionAvailable(msg.session.Region)
		m.selectedRegion = msg.session.Region
		m.rememberProfile(msg.session.Profile)
		m.rememberRegion(msg.session.Region)
		m.authPrompt = nil
		m.refreshProfileList(msg.session.Profile)
		m.refreshRegionList(msg.session.Region)
		m.statusMessage = fmt.Sprintf("Authenticated and activated profile %q in region %q.", msg.session.Profile, msg.session.Region)
		m.errorMessage = ""
		return m, m.currentPageStateCmd()
	case updateCheckedMsg:
		m.updateCheckBusy = false
		if msg.err != nil {
			if msg.startup {
				return m, nil
			}
			m.errorMessage = fmt.Sprintf("Could not check for updates: %v", msg.err)
			m.statusMessage = ""
			return m, nil
		}
		if msg.result.UpdateAvailable {
			result := msg.result
			m.updateAvailable = &result
			m.errorMessage = ""
			m.statusMessage = fmt.Sprintf("Update available: %s. Open commands (:) and run Update aws-terminal.", msg.result.LatestVersion)
			return m, nil
		}
		m.updateAvailable = nil
		if !msg.startup {
			m.errorMessage = ""
			if msg.result.DevelopmentBuild {
				m.statusMessage = fmt.Sprintf("Development build; latest release is %s.", msg.result.LatestVersion)
			} else {
				m.statusMessage = fmt.Sprintf("aws-terminal is up to date (%s).", msg.result.CurrentVersion)
			}
		}
		return m, nil
	case updateInstalledMsg:
		m.updateInstallBusy = false
		if msg.err != nil {
			m.errorMessage = fmt.Sprintf("Unable to update aws-terminal: %v", msg.err)
			m.statusMessage = ""
			return m, nil
		}
		m.errorMessage = ""
		if msg.result.Updated {
			m.updateAvailable = nil
			m.statusMessage = fmt.Sprintf("Updated aws-terminal to %s. Restart the app to use the new version.", msg.result.LatestVersion)
		} else if msg.result.SelfUpdatable {
			m.statusMessage = fmt.Sprintf("aws-terminal is up to date (%s).", msg.result.LatestVersion)
		} else {
			m.statusMessage = msg.result.Instructions
		}
		return m, nil
	case pages.OpenPageMsg:
		return m.openPage(msg.PageID, msg.Focus)
	case pages.OwnedMsg:
		return m.routeOwnedPageMsg(msg)
	case tea.KeyMsg:
		if m.paletteOpen {
			return m, m.updatePalette(msg)
		}
		if m.focus == focusPage && m.currentPageHasFocusedInput() && pageInputOwnsKey(msg) && msg.Type != tea.KeyCtrlC && !key.Matches(msg, m.keys.Focus) && !key.Matches(msg, m.keys.BackFocus) {
			return m, m.currentPageUpdateCmd(msg)
		}
		switch {
		case key.Matches(msg, m.keys.Palette):
			m.openPalette()
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			m.cancelProfileCommands()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Focus):
			return m, m.toggleFocus()
		case key.Matches(msg, m.keys.BackFocus):
			return m, m.toggleFocusBack()
		case key.Matches(msg, m.keys.Refresh):
			m.statusMessage = "Refreshing AWS profiles..."
			m.errorMessage = ""
			return m, m.loadProfilesCmd()
		}

		switch m.focus {
		case focusProfiles:
			if key.Matches(msg, m.keys.Select) {
				return m.activateSelectedProfile()
			}

			beforeName := ""
			if profile, ok := m.selectedProfile(); ok {
				beforeName = profile.Name
			}

			var cmd tea.Cmd
			m.profileList, cmd = m.profileList.Update(msg)
			afterProfile, ok := m.selectedProfile()
			if ok && afterProfile.Name != beforeName {
				m.ensureRegionAvailable(afterProfile.DefaultRegion)
				m.refreshRegionList("")
				return m, tea.Batch(cmd, m.currentPageStateCmd())
			}
			return m, cmd
		case focusRegions:
			if key.Matches(msg, m.keys.Select) {
				return m.selectRegion()
			}

			var cmd tea.Cmd
			m.regionList, cmd = m.regionList.Update(msg)
			return m, cmd
		case focusNavigation:
			if key.Matches(msg, m.keys.Select) {
				return m, m.setFocus(focusPage)
			}

			previousPageID := m.currentPageID()
			var cmd tea.Cmd
			m.pageList, cmd = m.pageList.Update(msg)
			return m.handlePageSelectionChange(previousPageID, cmd)
		case focusPage:
			return m, m.currentPageUpdateCmd(msg)
		}
	default:
		return m, m.currentPageUpdateCmd(msg)
	}

	return m, nil
}

func (m Model) currentPageHasFocusedInput() bool {
	provider, ok := m.currentPage().(pages.FocusedInputProvider)
	return ok && provider.HasFocusedInput()
}

func pageInputOwnsKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete, tea.KeyEsc, tea.KeyEnter:
		return true
	default:
		return false
	}
}

func (m *Model) toggleFocus() tea.Cmd {
	switch m.focus {
	case focusProfiles:
		return m.setFocus(focusRegions)
	case focusRegions:
		return m.setFocus(focusNavigation)
	case focusNavigation:
		return m.setFocus(focusProfiles)
	default:
		return m.setFocus(focusNavigation)
	}
}

func (m *Model) toggleFocusBack() tea.Cmd {
	switch m.focus {
	case focusProfiles:
		return m.setFocus(focusNavigation)
	case focusRegions:
		return m.setFocus(focusProfiles)
	case focusNavigation:
		return m.setFocus(focusRegions)
	default:
		return m.setFocus(focusNavigation)
	}
}

func (m *Model) setFocus(next focusArea) tea.Cmd {
	if m.focus == next {
		return nil
	}

	previousFocus := m.focus
	m.focus = next
	m.applySidebarListFocus()

	cmds := make([]tea.Cmd, 0, 2)
	if previousFocus == focusPage {
		cmds = append(cmds, m.currentPage().SetFocused(false))
	}
	if next == focusPage {
		cmds = append(cmds, m.currentPageStateCmd(), m.currentPage().SetFocused(true))
	}

	return tea.Batch(cmds...)
}

func (m Model) handlePageSelectionChange(previousPageID string, updateCmd tea.Cmd) (tea.Model, tea.Cmd) {
	if previousPageID == m.currentPageID() {
		return m, updateCmd
	}

	m.rememberPage(m.currentPageID())
	cmds := []tea.Cmd{updateCmd, m.currentPageStateCmd()}
	if m.focus == focusPage {
		if previousPage := m.pageByID(previousPageID); previousPage != nil {
			cmds = append(cmds, previousPage.SetFocused(false))
		}
		cmds = append(cmds, m.currentPage().SetFocused(true))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) routeOwnedPageMsg(msg pages.OwnedMsg) (tea.Model, tea.Cmd) {
	ownerPageID := msg.OwnerPageID()
	if ownerPageID == "" {
		return m, m.currentPageUpdateCmd(msg)
	}

	ownerPage := m.pageByID(ownerPageID)
	if ownerPage == nil {
		return m, nil
	}

	return m, ownerPage.Update(msg, m.pageState())
}

func (m Model) openPage(pageID string, focus bool) (tea.Model, tea.Cmd) {
	previousPageID := m.currentPageID()
	m.selectPageListItem(pageID)
	m.rememberPage(m.currentPageID())
	if previousPageID == m.currentPageID() && (!focus || m.focus == focusPage) {
		return m, nil
	}

	cmds := []tea.Cmd{m.currentPageStateCmd()}
	if previousPageID != m.currentPageID() && m.focus == focusPage {
		if previousPage := m.pageByID(previousPageID); previousPage != nil {
			cmds = append(cmds, previousPage.SetFocused(false))
		}
	}
	if focus {
		cmds = append(cmds, m.setFocus(focusPage))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) activateSelectedProfile() (tea.Model, tea.Cmd) {
	if m.profileBusy {
		return m, nil
	}

	profile, ok := m.selectedProfile()
	if !ok {
		m.errorMessage = "No AWS profile is available to authenticate."
		m.statusMessage = ""
		return m, nil
	}

	m.profileBusy = true
	m.errorMessage = ""
	m.authPrompt = nil
	m.refreshProfileList(profile.Name)
	if profile.UsesSSO() {
		m.statusMessage = fmt.Sprintf("Checking cached AWS SSO session for profile %q...", profile.Name)
		return m, tea.Batch(m.spinner.Tick, m.checkSSOSessionCmd(profile), m.currentPageStateCmd())
	}

	m.rememberProfile(profile.Name)
	m.statusMessage = fmt.Sprintf("Loading credentials for profile %q in region %q...", profile.Name, m.activeRegion())
	return m, tea.Batch(m.activateProfileCmd(profile.Name), m.currentPageStateCmd())
}

func (m Model) selectRegion() (tea.Model, tea.Cmd) {
	region, ok := m.selectedRegionEntry()
	if !ok {
		return m, nil
	}

	m.selectedRegion = region.ID
	m.rememberRegion(region.ID)
	m.errorMessage = ""
	m.refreshRegionList(region.ID)
	if m.activeSession != nil {
		m.activeSession.Region = region.ID
		m.statusMessage = fmt.Sprintf("Active region switched to %q for profile %q.", region.ID, m.activeSession.Profile)
	} else {
		m.statusMessage = fmt.Sprintf("Selected region %q. Authenticate a profile to use it.", region.ID)
	}

	return m, m.currentPageStateCmd()
}

func (m *Model) cancelProfileCommands() {
	if m.loadProfilesCancel != nil {
		m.loadProfilesCancel()
		m.loadProfilesCancel = nil
	}
	if m.checkSSOSessionCancel != nil {
		m.checkSSOSessionCancel()
		m.checkSSOSessionCancel = nil
	}
	if m.startSSOLoginCancel != nil {
		m.startSSOLoginCancel()
		m.startSSOLoginCancel = nil
	}
	if m.pollSSOLoginCancel != nil {
		m.pollSSOLoginCancel()
		m.pollSSOLoginCancel = nil
	}
	if m.activateProfileCancel != nil {
		m.activateProfileCancel()
		m.activateProfileCancel = nil
	}
}

func (m *Model) loadProfilesCmd() tea.Cmd {
	if m.loadProfilesCancel != nil {
		m.loadProfilesCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.loadProfilesCancel = cancel
	return func() tea.Msg {
		profiles, err := m.sessionService.ListProfiles(ctx)
		return profilesLoadedMsg{profiles: profiles, err: err}
	}
}

func (m *Model) checkSSOSessionCmd(profile domainprofile.Profile) tea.Cmd {
	if m.checkSSOSessionCancel != nil {
		m.checkSSOSessionCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.checkSSOSessionCancel = cancel
	return func() tea.Msg {
		reusable, err := m.authService.HasReusableSSOSession(ctx, profile)
		return ssoSessionCheckedMsg{profile: profile, reusable: reusable, err: err}
	}
}

func (m *Model) startSSOLoginCmd(profile domainprofile.Profile) tea.Cmd {
	if m.startSSOLoginCancel != nil {
		m.startSSOLoginCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.startSSOLoginCancel = cancel
	return func() tea.Msg {
		prompt, err := m.authService.StartSSOLogin(ctx, profile)
		return ssoLoginStartedMsg{prompt: prompt, err: err}
	}
}

func (m *Model) pollSSOLoginCmd(sessionID string, delay time.Duration) tea.Cmd {
	if m.pollSSOLoginCancel != nil {
		m.pollSSOLoginCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.pollSSOLoginCancel = cancel
	return func() tea.Msg {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ssoLoginPolledMsg{sessionID: sessionID, err: ctx.Err()}
			case <-timer.C:
			}
		}
		result, err := m.authService.PollSSOLogin(ctx, sessionID)
		return ssoLoginPolledMsg{sessionID: sessionID, result: result, err: err}
	}
}

func (m *Model) activateProfileCmd(profileName string) tea.Cmd {
	if m.activateProfileCancel != nil {
		m.activateProfileCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.activateProfileCancel = cancel
	region := m.activeRegion()
	return func() tea.Msg {
		session, err := m.sessionService.ActivateProfile(ctx, profileName, region)
		return profileActivatedMsg{session: session, err: err}
	}
}

func (m Model) currentPageStateCmd() tea.Cmd {
	return m.currentPage().OnStateChanged(m.pageState())
}

func (m Model) currentPageUpdateCmd(msg tea.Msg) tea.Cmd {
	return m.currentPage().Update(msg, m.pageState())
}
