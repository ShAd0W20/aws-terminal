package shell

import (
	"aws-terminal/internal/ui/pageapi"
	"strings"
	"testing"
)

type statusTestPage struct {
	testPage
	status pageapi.Status
}

func (p *statusTestPage) PageStatus(pageapi.State) pageapi.Status {
	return p.status
}

func TestCurrentPageStatusUsesOptionalProvider(t *testing.T) {
	page := &statusTestPage{testPage: testPage{id: "status"}, status: pageapi.Status{Message: "workflow running"}}
	model := Model{pageRegistry: []pageapi.Page{page}, pageList: newSidebarListModel()}
	model.refreshPageList("status")

	status := model.currentPageStatus()
	if status.Message != "workflow running" {
		t.Fatalf("expected workflow status, got %#v", status)
	}
}

func TestFooterIncludesPageStatusSeparatelyFromGlobalStatus(t *testing.T) {
	page := &statusTestPage{testPage: testPage{id: "status"}, status: pageapi.Status{Message: "workflow running"}}
	model := Model{
		width:         120,
		height:        40,
		pageRegistry:  []pageapi.Page{page},
		pageList:      newSidebarListModel(),
		profileList:   newSidebarListModel(),
		regionList:    newSidebarListModel(),
		statusMessage: "global auth status",
	}
	model.refreshPageList("status")

	footer := model.footerView(120)
	if !strings.Contains(footer, "Page status: workflow running") {
		t.Fatalf("expected page status in footer, got:\n%s", footer)
	}
	if strings.Contains(footer, "global auth status") {
		t.Fatalf("did not expect global shell status to be mixed into footer status parts, got:\n%s", footer)
	}
}
