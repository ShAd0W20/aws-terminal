package shell

import (
	"aws-terminal/internal/ui/pageapi"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

type pageListItem struct {
	Page pageapi.Page
}

func (i pageListItem) FilterValue() string {
	return strings.TrimSpace(i.Page.Title() + " " + i.Page.Description())
}

func (i pageListItem) SidebarLabel() string {
	return i.Page.Title()
}

func (i pageListItem) SidebarActive() bool {
	return false
}

func (m *Model) refreshPageList(targetPageID string) {
	if strings.TrimSpace(targetPageID) == "" {
		if item, ok := m.pageList.SelectedItem().(pageListItem); ok {
			targetPageID = item.Page.ID()
		}
	}

	items := make([]list.Item, 0, len(m.pageRegistry))
	for _, page := range m.pageRegistry {
		items = append(items, pageListItem{Page: page})
	}
	_ = m.pageList.SetItems(items)

	if strings.TrimSpace(targetPageID) != "" {
		m.selectPageListItem(targetPageID)
	} else if len(items) > 0 {
		m.pageList.ResetSelected()
	}

	m.applySidebarListFocus()
	m.syncSidebarListsLayout()
}

func (m *Model) selectPageListItem(pageID string) {
	for index, page := range m.pageRegistry {
		if page.ID() == strings.TrimSpace(pageID) {
			m.pageList.Select(index)
			return
		}
	}

	if len(m.pageRegistry) > 0 {
		m.pageList.ResetSelected()
	}
}
