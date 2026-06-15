package shell

import (
	"strings"
	"testing"
)

func TestSidebarAutoCollapsesForFocusedPageOnMacBookSizedTerminal(t *testing.T) {
	if !shouldCollapseSidebar(146, 39, true) {
		t.Fatal("expected focused page to collapse sidebar at 150x43-ish inner size")
	}
	if shouldCollapseSidebar(146, 39, false) {
		t.Fatal("expected sidebar to remain visible while navigation has focus")
	}
	if shouldCollapseSidebar(180, 50, true) {
		t.Fatal("expected roomy terminals to keep sidebar visible")
	}
}

func TestCompactFooterKeepsOneLineHelpAndNavigationHint(t *testing.T) {
	model := newTestShellModelWithPages("ecs")
	model.width = 150
	model.height = 43
	model.ready = true
	model.focus = focusPage

	footer := model.footerView(146)
	if !strings.Contains(footer, "tab nav") {
		t.Fatalf("expected compact footer to explain navigation return, got:\n%s", footer)
	}
	if strings.Contains(footer, "use page keys") {
		t.Fatalf("expected useful one-line bindings instead of vague compact help, got:\n%s", footer)
	}
}
