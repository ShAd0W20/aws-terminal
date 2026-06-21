package shell

import (
	"aws-terminal/internal/ui/pageapi"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPaletteOpensAndCloses(t *testing.T) {
	model := newTestShellModelWithPages("dashboard")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	model = updated.(Model)
	if !model.paletteOpen {
		t.Fatal("expected palette open")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.paletteOpen {
		t.Fatal("expected palette closed")
	}
}

func TestPaletteRunsOpenPageAction(t *testing.T) {
	model := newTestShellModelWithPages("dashboard", "s3")
	model.openPalette()
	actions := model.quickActions()
	for index, action := range actions {
		if action.Title == "Open s3" {
			model.paletteIndex = index
			break
		}
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		_ = cmd()
	}
	if model.paletteOpen {
		t.Fatal("expected palette closed after action")
	}
	if got := model.currentPageID(); got != "s3" {
		t.Fatalf("current page = %q, want s3", got)
	}
	if model.focus != focusPage {
		t.Fatalf("focus = %v, want focusPage", model.focus)
	}
}

func TestPaletteViewRendersActions(t *testing.T) {
	model := Model{
		pageRegistry: []pageapi.Page{&testPage{id: "dashboard"}},
		pageList:     newSidebarListModel(),
		paletteOpen:  true,
	}
	model.refreshPageList("dashboard")

	view := model.paletteView(80, 20)
	if view == "" {
		t.Fatal("expected palette view")
	}
}
