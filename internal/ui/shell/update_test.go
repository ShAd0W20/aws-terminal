package shell

import (
	"aws-terminal/internal/ui/pageapi"
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	domainauth "aws-terminal/internal/domain/auth"
	domainprofile "aws-terminal/internal/domain/profile"
	domainsession "aws-terminal/internal/domain/session"
)

type testOwnedMsg struct{ pageID string }

func (m testOwnedMsg) OwnerPageID() string { return m.pageID }

type testPage struct {
	id      string
	updates int
	lastMsg tea.Msg
}

func (p *testPage) ID() string                           { return p.id }
func (p *testPage) Title() string                        { return p.id }
func (p *testPage) Description() string                  { return p.id }
func (p *testPage) OnStateChanged(pageapi.State) tea.Cmd { return nil }
func (p *testPage) SetFocused(bool) tea.Cmd              { return nil }
func (p *testPage) Update(msg tea.Msg, _ pageapi.State) tea.Cmd {
	p.updates++
	p.lastMsg = msg
	return nil
}
func (p *testPage) View(pageapi.State, int, int) string { return "" }
func (p *testPage) ShortHelp() []key.Binding            { return nil }
func (p *testPage) FullHelp() [][]key.Binding           { return nil }

func TestOwnedPageMessageRoutesToOwnerWhenNotCurrentPage(t *testing.T) {
	current := &testPage{id: "current"}
	owner := &testPage{id: "owner"}
	model := Model{
		pageRegistry: []pageapi.Page{current, owner},
		pageList:     newSidebarListModel(),
	}
	model.refreshPageList("current")

	updated, cmd := model.Update(testOwnedMsg{pageID: "owner"})
	if cmd != nil {
		t.Fatalf("expected no command, got %T", cmd)
	}
	if _, ok := updated.(Model); !ok {
		t.Fatalf("expected shell Model, got %T", updated)
	}
	if owner.updates != 1 {
		t.Fatalf("expected owner page to receive 1 update, got %d", owner.updates)
	}
	if current.updates != 0 {
		t.Fatalf("expected current page not to receive update, got %d", current.updates)
	}
	if _, ok := owner.lastMsg.(testOwnedMsg); !ok {
		t.Fatalf("expected owner page to receive testOwnedMsg, got %T", owner.lastMsg)
	}
}

type shellTestSessionService struct{}

func (shellTestSessionService) ListProfiles(context.Context) ([]domainprofile.Profile, error) {
	return nil, nil
}

func (shellTestSessionService) ActivateProfile(_ context.Context, profileName, region string) (domainsession.Session, error) {
	return domainsession.Session{Profile: profileName, Region: region}, nil
}

type shellTestAuthService struct {
	reusable bool
}

func (s shellTestAuthService) HasReusableSSOSession(context.Context, domainprofile.Profile) (bool, error) {
	return s.reusable, nil
}

func (shellTestAuthService) StartSSOLogin(_ context.Context, profile domainprofile.Profile) (domainauth.Prompt, error) {
	return domainauth.Prompt{SessionID: "session", ProfileName: profile.Name}, nil
}

func (shellTestAuthService) PollSSOLogin(context.Context, string) (domainauth.PollResult, error) {
	return domainauth.PollResult{}, nil
}

func TestRefreshKeyRoutesToPageWhenPageFocused(t *testing.T) {
	page := &testPage{id: "current"}
	model := Model{
		sessionService: shellTestSessionService{},
		authService:    shellTestAuthService{},
		keys:           DefaultKeyMap,
		pageRegistry:   []pageapi.Page{page},
		pageList:       newSidebarListModel(),
		profileList:    newSidebarListModel(),
		focus:          focusPage,
	}
	model.refreshPageList("current")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model = updated.(Model)
	if cmd != nil {
		cmd()
	}
	if page.updates != 1 {
		t.Fatalf("expected focused page to receive refresh key, got %d updates", page.updates)
	}
	if keyMsg, ok := page.lastMsg.(tea.KeyMsg); !ok || string(keyMsg.Runes) != "r" {
		t.Fatalf("expected page to receive r key, got %#v", page.lastMsg)
	}
	if model.statusMessage == "Refreshing AWS profiles..." {
		t.Fatal("refresh key should not trigger global profile refresh while page is focused")
	}
}

func TestActivateSelectedSSOProfileChecksReusableSession(t *testing.T) {
	profile := testShellSSOProfile()
	model := testShellModel(shellTestAuthService{reusable: true}, profile)

	updated, cmd := model.activateSelectedProfile()
	model = updated.(Model)
	if !model.profileBusy {
		t.Fatal("expected profile to be busy")
	}
	if cmd == nil {
		t.Fatal("expected command")
	}

	msg := firstNonNilBatchMessage(t, cmd)
	checked, ok := msg.(ssoSessionCheckedMsg)
	if !ok {
		t.Fatalf("expected ssoSessionCheckedMsg, got %T", msg)
	}
	if !checked.reusable || checked.profile.Name != profile.Name {
		t.Fatalf("unexpected check message: %#v", checked)
	}
}

func TestReusableSSOSessionActivatesWithoutStartingLogin(t *testing.T) {
	profile := testShellSSOProfile()
	model := testShellModel(shellTestAuthService{reusable: true}, profile)
	model.profileBusy = true

	updated, cmd := model.Update(ssoSessionCheckedMsg{profile: profile, reusable: true})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected activation command")
	}

	msg := firstNonNilBatchMessage(t, cmd)
	activated, ok := msg.(profileActivatedMsg)
	if !ok {
		t.Fatalf("expected profileActivatedMsg, got %T", msg)
	}
	if activated.session.Profile != profile.Name {
		t.Fatalf("activated profile = %q, want %q", activated.session.Profile, profile.Name)
	}
}

func TestNonReusableSSOSessionStartsDeviceLogin(t *testing.T) {
	profile := testShellSSOProfile()
	model := testShellModel(shellTestAuthService{}, profile)
	model.profileBusy = true

	_, cmd := model.Update(ssoSessionCheckedMsg{profile: profile, reusable: false})
	if cmd == nil {
		t.Fatal("expected login command")
	}

	msg := firstNonNilBatchMessage(t, cmd)
	started, ok := msg.(ssoLoginStartedMsg)
	if !ok {
		t.Fatalf("expected ssoLoginStartedMsg, got %T", msg)
	}
	if started.prompt.ProfileName != profile.Name {
		t.Fatalf("started profile = %q, want %q", started.prompt.ProfileName, profile.Name)
	}
}

func testShellModel(auth AuthenticationService, profile domainprofile.Profile) Model {
	page := &testPage{id: "current"}
	model := Model{
		sessionService: shellTestSessionService{},
		authService:    auth,
		pageRegistry:   []pageapi.Page{page},
		pageList:       newSidebarListModel(),
		profileList:    newSidebarListModel(),
		profiles:       []domainprofile.Profile{profile},
		selectedRegion: "eu-west-1",
	}
	model.refreshPageList("current")
	model.refreshProfileList(profile.Name)
	return model
}

func testShellSSOProfile() domainprofile.Profile {
	return domainprofile.Profile{
		Name:               "sso-dev",
		AuthenticationMode: domainprofile.AuthModeSSO,
		SSO: &domainprofile.SSOConfiguration{
			StartURL: "https://example.awsapps.com/start",
			Region:   "us-east-1",
		},
	}
}

func firstNonNilBatchMessage(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return msg
	}
	for _, nested := range batch {
		if nested == nil {
			continue
		}
		if nestedMsg := nested(); nestedMsg != nil {
			switch nestedMsg.(type) {
			case spinner.TickMsg:
				continue
			default:
				return nestedMsg
			}
		}
	}
	return nil
}

func TestOwnedPageMessageForUnknownOwnerIsIgnored(t *testing.T) {
	current := &testPage{id: "current"}
	model := Model{
		pageRegistry: []pageapi.Page{current},
		pageList:     newSidebarListModel(),
	}
	model.refreshPageList("current")

	_, cmd := model.Update(testOwnedMsg{pageID: "missing"})
	if cmd != nil {
		t.Fatalf("expected no command, got %T", cmd)
	}
	if current.updates != 0 {
		t.Fatalf("expected current page not to receive unknown-owner message, got %d", current.updates)
	}
}
