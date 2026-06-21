package ecr

import (
	"aws-terminal/internal/ui/pageapi"
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	appsecr "aws-terminal/internal/application/ecr"
	domainecr "aws-terminal/internal/domain/ecr"
	domainsession "aws-terminal/internal/domain/session"
)

type fakeECRService struct {
	repositories []domainecr.Repository
	created      domainecr.Repository
	images       []domainecr.RepositoryImage
	locals       []domainecr.LocalImage
}

func (f *fakeECRService) ListRepositories(ctx context.Context, profileName, region string) ([]domainecr.Repository, error) {
	return f.repositories, nil
}
func (f *fakeECRService) CreateRepository(ctx context.Context, input domainecr.CreateRepositoryInput) (domainecr.Repository, error) {
	if f.created.Name != "" {
		return f.created, nil
	}
	return domainecr.Repository{Name: input.Name, URI: "123.dkr.ecr." + input.Region + ".amazonaws.com/" + input.Name}, nil
}
func (f *fakeECRService) ListImages(ctx context.Context, profileName, region, repositoryName string) ([]domainecr.RepositoryImage, error) {
	return f.images, nil
}
func (f *fakeECRService) ListLocalImages(ctx context.Context) ([]domainecr.LocalImage, error) {
	return f.locals, nil
}
func (f *fakeECRService) BuildPushPlan(input appsecr.BuildPushPlanInput) (domainecr.PushPlan, error) {
	return domainecr.PushPlan{Profile: input.Profile, Region: input.Region, RepositoryName: input.RepositoryName, RepositoryURI: input.RepositoryURI, SourceImage: input.SourceImage, DestinationTag: input.DestinationTag, DestinationImage: input.RepositoryURI + ":" + input.DestinationTag}, nil
}
func (f *fakeECRService) ExecutePush(ctx context.Context, plan domainecr.PushPlan, progress chan<- domainecr.PushProgress) (domainecr.PushResult, error) {
	return domainecr.PushResult{DestinationImage: plan.DestinationImage}, nil
}

func testState() pageapi.State {
	return pageapi.State{ActiveSession: &domainsession.Session{Profile: "pre", Region: "eu-west-1", Account: "123"}, SelectedRegion: "eu-west-1", PageFocused: true}
}

func TestRepositorySearchAcceptsKeybindLetters(t *testing.T) {
	p := NewECRPage(&fakeECRService{})
	state := testState()
	p.stage = ecrStageRepository

	p.Update(tea.KeyMsg{Type: tea.KeyCtrlF}, state)
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}, state)
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}, state)
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}, state)

	if p.searchInput.Value() != "crb" {
		t.Fatalf("search input did not receive keybind letters: %q", p.searchInput.Value())
	}
	if p.stage != ecrStageRepository {
		t.Fatalf("stage changed while typing: %v", p.stage)
	}
}

func TestRepositoryEnterAfterSearchClampsSelection(t *testing.T) {
	p := NewECRPage(&fakeECRService{repositories: []domainecr.Repository{{Name: "api", URI: "uri/api"}, {Name: "web", URI: "uri/web"}}})
	state := testState()
	p.repositories = []domainecr.Repository{{Name: "api", URI: "uri/api"}, {Name: "web", URI: "uri/web"}}
	p.repositoryIndex = 1
	p.searchInput.Focus()
	p.searchInput.SetValue("api")

	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if p.selectedRepository.Name != "api" {
		t.Fatalf("selected repository = %q", p.selectedRepository.Name)
	}
	if cmd == nil {
		t.Fatal("expected load images command")
	}
}

func TestRepositorySelectionLoadsImages(t *testing.T) {
	p := NewECRPage(&fakeECRService{repositories: []domainecr.Repository{{Name: "api", URI: "123.dkr.ecr.eu-west-1.amazonaws.com/api"}}})
	state := testState()
	cmd := p.OnStateChanged(state)
	if cmd == nil {
		t.Fatal("expected load command")
	}
	p.Update(cmd(), state)
	if len(p.repositories) != 1 {
		t.Fatalf("repositories not loaded: %#v", p.repositories)
	}
	cmd = p.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if p.stage != ecrStageRepositoryImages {
		t.Fatalf("stage=%v", p.stage)
	}
	if cmd == nil {
		t.Fatal("expected images load command")
	}
}

func TestLocalImageListWindowFollowsSelection(t *testing.T) {
	p := NewECRPage(&fakeECRService{})
	p.stage = ecrStageLocalImage
	for i := 0; i < 25; i++ {
		p.localImages = append(p.localImages, domainecr.LocalImage{Reference: fmt.Sprintf("image:%02d", i)})
	}
	p.localIndex = 20

	view := strings.Join(p.localLines(120, 20, 0), "\n")
	if !strings.Contains(view, "image:20") {
		t.Fatalf("selected image was not visible: %s", view)
	}
	if strings.Contains(view, "image:00") {
		t.Fatalf("list did not scroll away from the first page: %s", view)
	}
}

func TestSuccessfulPushResetsToRepositoryImages(t *testing.T) {
	p := NewECRPage(&fakeECRService{})
	state := testState()
	p.stage = ecrStagePush
	p.pushing = true
	p.selectedRepository = domainecr.Repository{Name: "backend-api", URI: "123.dkr.ecr.eu-west-1.amazonaws.com/backend-api"}
	p.localImages = []domainecr.LocalImage{{Reference: "backend-api:4.1.1"}}
	p.manualInput.SetValue("backend-api:4.1.1")
	p.tagInput.SetValue("4.1.1")
	p.plan = &domainecr.PushPlan{DestinationImage: "123.dkr.ecr.eu-west-1.amazonaws.com/backend-api:4.1.1"}

	cmd := p.Update(pushEventMsg{event: pushEvent{done: true, result: &domainecr.PushResult{DestinationImage: "123.dkr.ecr.eu-west-1.amazonaws.com/backend-api:4.1.1"}}}, state)
	if p.stage != ecrStageRepositoryImages {
		t.Fatalf("stage=%v", p.stage)
	}
	if p.selectedSourceImage() != "" || p.plan != nil || p.pushing {
		t.Fatalf("push state not reset: source=%q plan=%#v pushing=%v", p.selectedSourceImage(), p.plan, p.pushing)
	}
	if p.pushMessage == "" {
		t.Fatal("expected push success message")
	}
	if cmd == nil {
		t.Fatal("expected image refresh command after push")
	}
}

func TestCreateRepositoryTransition(t *testing.T) {
	p := NewECRPage(&fakeECRService{})
	state := testState()
	p.stage = ecrStageRepository
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, state)
	if p.stage != ecrStageCreateRepository {
		t.Fatalf("stage=%v", p.stage)
	}
	p.createInput.SetValue("new-repo")
	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if cmd == nil {
		t.Fatal("expected create command")
	}
	p.Update(repositoryCreatedMsg{repository: domainecr.Repository{Name: "new-repo", URI: "123.dkr.ecr.eu-west-1.amazonaws.com/new-repo"}}, state)
	if p.selectedRepository.Name != "new-repo" || p.stage != ecrStageRepositoryImages {
		t.Fatalf("selected=%#v stage=%v", p.selectedRepository, p.stage)
	}
}
