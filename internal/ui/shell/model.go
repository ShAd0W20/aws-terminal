package shell

import (
	"context"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"aws-terminal/internal/config"
	domainauth "aws-terminal/internal/domain/auth"
	domainprofile "aws-terminal/internal/domain/profile"
	domainregion "aws-terminal/internal/domain/region"
	domainsession "aws-terminal/internal/domain/session"
	domainupdate "aws-terminal/internal/domain/update"
	"aws-terminal/internal/ui/pages"
	"aws-terminal/internal/ui/styles"
)

type SessionService interface {
	ListProfiles(ctx context.Context) ([]domainprofile.Profile, error)
	ActivateProfile(ctx context.Context, profileName, region string) (domainsession.Session, error)
}

type AuthenticationService interface {
	HasReusableSSOSession(ctx context.Context, profile domainprofile.Profile) (bool, error)
	StartSSOLogin(ctx context.Context, profile domainprofile.Profile) (domainauth.Prompt, error)
	PollSSOLogin(ctx context.Context, sessionID string) (domainauth.PollResult, error)
}

type UpdateService interface {
	CurrentVersion() string
	Check(ctx context.Context) (domainupdate.CheckResult, error)
	InstallLatest(ctx context.Context) (domainupdate.InstallResult, error)
}

type focusArea int

const (
	focusProfiles focusArea = iota
	focusRegions
	focusNavigation
	focusPage
)

type Model struct {
	sessionService        SessionService
	authService           AuthenticationService
	width                 int
	height                int
	ready                 bool
	help                  help.Model
	spinner               spinner.Model
	keys                  KeyMap
	pageRegistry          []pages.Page
	profiles              []domainprofile.Profile
	profileList           list.Model
	regions               []domainregion.Region
	regionList            list.Model
	pageList              list.Model
	selectedRegion        string
	focus                 focusArea
	activeSession         *domainsession.Session
	profileBusy           bool
	loadProfilesCancel    context.CancelFunc
	checkSSOSessionCancel context.CancelFunc
	startSSOLoginCancel   context.CancelFunc
	pollSSOLoginCancel    context.CancelFunc
	activateProfileCancel context.CancelFunc
	statusMessage         string
	errorMessage          string
	authPrompt            *domainauth.Prompt
	preferenceStore       config.PreferenceStore
	preferences           config.Preferences
	updateService         UpdateService
	updateCheckBusy       bool
	updateInstallBusy     bool
	updateAvailable       *domainupdate.CheckResult
	paletteOpen           bool
	paletteIndex          int
}

func NewModel(sessionService SessionService, authService AuthenticationService, pageRegistry []pages.Page) Model {
	return NewModelWithPreferences(sessionService, authService, pageRegistry, nil)
}

func NewModelWithPreferences(sessionService SessionService, authService AuthenticationService, pageRegistry []pages.Page, preferenceStore config.PreferenceStore) Model {
	return NewModelWithPreferencesAndUpdates(sessionService, authService, pageRegistry, preferenceStore, nil)
}

func NewModelWithPreferencesAndUpdates(sessionService SessionService, authService AuthenticationService, pageRegistry []pages.Page, preferenceStore config.PreferenceStore, updateService UpdateService) Model {
	helpModel := help.New()
	helpModel.ShowAll = false

	spinnerModel := spinner.New()
	spinnerModel.Spinner = spinner.Dot
	spinnerModel.Style = styles.StatusStyle

	preferences := config.Preferences{}
	if preferenceStore != nil {
		if loaded, err := preferenceStore.Load(); err == nil {
			preferences = loaded
		}
		_ = preferenceStore.Save(preferences)
	}

	model := Model{
		sessionService:  sessionService,
		authService:     authService,
		updateService:   updateService,
		preferenceStore: preferenceStore,
		preferences:     preferences,
		help:            helpModel,
		spinner:         spinnerModel,
		keys:            DefaultKeyMap,
		pageRegistry:    pageRegistry,
		profileList:     newSidebarListModel(),
		regions:         domainregion.DefaultCatalog(),
		regionList:      newSidebarListModel(),
		pageList:        newSidebarListModel(),
		focus:           focusProfiles,
	}
	model.ensureRegionAvailable(preferences.LastRegion)
	model.selectedRegion = preferences.LastRegion
	model.refreshPageList(preferences.LastPage)
	model.syncRegionSelection()
	model.refreshProfileList("")
	model.applySidebarListFocus()
	return model
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.loadProfilesCmd(), m.currentPage().OnStateChanged(m.pageState())}
	if m.shouldCheckUpdatesOnStart() {
		cmds = append(cmds, m.checkUpdatesCmd(true))
	}
	return tea.Batch(cmds...)
}

func (m Model) innerWidth() int {
	return m.width - styles.DocStyle.GetHorizontalFrameSize()
}

func (m Model) innerHeight() int {
	return m.height - styles.DocStyle.GetVerticalFrameSize()
}

func (m Model) currentPage() pages.Page {
	if item, ok := m.pageList.SelectedItem().(pageListItem); ok {
		return item.Page
	}
	if len(m.pageRegistry) == 0 {
		return pages.NewDashboardPage()
	}

	return m.pageRegistry[0]
}

func (m Model) pageByID(pageID string) pages.Page {
	pageID = strings.TrimSpace(pageID)
	for _, page := range m.pageRegistry {
		if page.ID() == pageID {
			return page
		}
	}
	return nil
}

func (m Model) currentPageID() string {
	return m.currentPage().ID()
}

func (m Model) selectedProfile() (domainprofile.Profile, bool) {
	item, ok := m.profileList.SelectedItem().(profileListItem)
	if !ok {
		return domainprofile.Profile{}, false
	}

	return item.Profile, true
}

func (m Model) selectedProfilePointer() *domainprofile.Profile {
	profile, ok := m.selectedProfile()
	if !ok {
		return nil
	}

	selected := profile
	return &selected
}

func (m Model) selectedRegionEntry() (domainregion.Region, bool) {
	item, ok := m.regionList.SelectedItem().(regionListItem)
	if !ok {
		return domainregion.Region{}, false
	}

	return item.Region, true
}

func (m Model) pageState() pages.State {
	return pages.State{
		HighlightedProfile: m.selectedProfilePointer(),
		ActiveSession:      m.activeSession,
		SelectedRegion:     m.selectedRegion,
		StatusMessage:      m.statusMessage,
		ErrorMessage:       m.errorMessage,
		ProfileBusy:        m.profileBusy,
		AuthPrompt:         m.authPrompt,
		PageFocused:        m.focus == focusPage,
	}
}

func (m Model) preferredProfileNames() []string {
	preferred := []string{}
	if active := m.activeProfileName(); active != "" {
		preferred = append(preferred, active)
	}

	preferred = append(preferred,
		strings.TrimSpace(m.preferences.LastProfile),
		strings.TrimSpace(os.Getenv("AWS_PROFILE")),
		"default",
	)

	return preferred
}

func (m Model) preferredRegionNames() []string {
	preferred := []string{}
	if active := m.activeRegion(); active != "" {
		preferred = append(preferred, active)
	}

	preferred = append(preferred,
		strings.TrimSpace(m.preferences.LastRegion),
		strings.TrimSpace(os.Getenv("AWS_REGION")),
		strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION")),
	)

	if profile, ok := m.selectedProfile(); ok {
		preferred = append(preferred, strings.TrimSpace(profile.DefaultRegion))
	}

	preferred = append(preferred, "eu-west-1")
	return preferred
}

func (m *Model) ensureRegionAvailable(regionID string) {
	regionID = strings.TrimSpace(regionID)
	if regionID == "" {
		return
	}
	if _, found := m.regionIndex(regionID); found {
		return
	}

	m.regions = append(m.regions, domainregion.Region{ID: regionID})
}

func (m Model) regionIndex(regionID string) (int, bool) {
	for index, region := range m.regions {
		if region.ID == regionID {
			return index, true
		}
	}

	return 0, false
}

func (m Model) activeProfileName() string {
	if m.activeSession == nil {
		return ""
	}

	return m.activeSession.Profile
}

func (m Model) activeRegion() string {
	if strings.TrimSpace(m.selectedRegion) != "" {
		return strings.TrimSpace(m.selectedRegion)
	}
	if m.activeSession != nil && strings.TrimSpace(m.activeSession.Region) != "" {
		return strings.TrimSpace(m.activeSession.Region)
	}

	return ""
}

func (m Model) focusLabel() string {
	switch m.focus {
	case focusProfiles:
		return "Profiles"
	case focusRegions:
		return "Regions"
	case focusNavigation:
		return "Pages"
	default:
		return "Page"
	}
}
