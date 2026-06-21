package shell

import (
	"aws-terminal/internal/ui/pageapi"
	"testing"
)

func newTestShellModelWithPages(pageIDs ...string) Model {
	registry := make([]pageapi.Page, 0, len(pageIDs))
	for _, id := range pageIDs {
		registry = append(registry, &testPage{id: id})
	}
	model := Model{
		pageRegistry: registry,
		profileList:  newSidebarListModel(),
		regionList:   newSidebarListModel(),
		pageList:     newSidebarListModel(),
		keys:         DefaultKeyMap,
		focus:        focusProfiles,
	}
	model.refreshPageList("")
	model.applySidebarListFocus()
	return model
}

func TestFocusCyclesForwardAndBackward(t *testing.T) {
	model := newTestShellModelWithPages("dashboard")

	if model.focus != focusProfiles {
		t.Fatalf("expected initial focusProfiles, got %v", model.focus)
	}

	_ = model.toggleFocus()
	if model.focus != focusRegions {
		t.Fatalf("expected focusRegions, got %v", model.focus)
	}

	_ = model.toggleFocus()
	if model.focus != focusNavigation {
		t.Fatalf("expected focusNavigation, got %v", model.focus)
	}

	_ = model.toggleFocus()
	if model.focus != focusProfiles {
		t.Fatalf("expected wrap to focusProfiles, got %v", model.focus)
	}

	_ = model.toggleFocusBack()
	if model.focus != focusNavigation {
		t.Fatalf("expected backward wrap to focusNavigation, got %v", model.focus)
	}

	_ = model.toggleFocusBack()
	if model.focus != focusRegions {
		t.Fatalf("expected focusRegions, got %v", model.focus)
	}
}

func TestOpenPageSelectsPageAndCanFocusWorkflow(t *testing.T) {
	model := newTestShellModelWithPages("dashboard", "s3")
	if got := model.currentPageID(); got != "dashboard" {
		t.Fatalf("expected dashboard selected, got %q", got)
	}

	updated, _ := model.openPage("s3", true)
	model, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updated)
	}
	if got := model.currentPageID(); got != "s3" {
		t.Fatalf("expected s3 selected, got %q", got)
	}
	if model.focus != focusPage {
		t.Fatalf("expected focusPage, got %v", model.focus)
	}
}
