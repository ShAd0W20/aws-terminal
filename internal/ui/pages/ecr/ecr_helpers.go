package ecr

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	domainecr "aws-terminal/internal/domain/ecr"
	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
)

func ecrImageTableColumnsForWidth(width int) []table.Column {
	return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Tags", Min: 18, Weight: 4, Max: 56}, {Title: "Size", Min: 10}, {Title: "Created", Min: 16}, {Title: "Last pulled", Min: 16}, {Title: "Digest", Min: 20, Weight: 2, Max: 40}})
}

func ecrLocalImageTableColumnsForWidth(width int) []table.Column {
	return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Image", Min: 28, Weight: 5, Max: 88}, {Title: "Size", Min: 10}, {Title: "Created", Min: 16}, {Title: "ID", Min: 20, Weight: 2, Max: 40}})
}

func ecrImageTableColumns() []table.Column      { return ecrImageTableColumnsForWidth(112) }
func ecrLocalImageTableColumns() []table.Column { return ecrLocalImageTableColumnsForWidth(120) }

func ecrTableStyles() table.Styles {
	tableStyles := table.DefaultStyles()
	tableStyles.Header = tableStyles.Header.Bold(true).Foreground(lipgloss.Color("39"))
	tableStyles.Selected = styles.FocusedSelectedSidebarItemStyle
	return tableStyles
}

func ecrTextInputKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete:
		return true
	default:
		return false
	}
}

func (p *ECRPage) configureImageTable(width, rows int) {
	rows = max(5, rows)
	p.imagePaginator.PerPage = rows
	p.imageTable.SetHeight(rows + 1)
	p.imageTable.SetWidth(width)
	p.imageTable.SetColumns(ecrImageTableColumnsForWidth(width))
}

func (p *ECRPage) configureLocalTable(width, rows int) {
	rows = max(5, rows)
	p.localPaginator.PerPage = rows
	p.localTable.SetHeight(rows + 1)
	p.localTable.SetWidth(width)
	p.localTable.SetColumns(ecrLocalImageTableColumnsForWidth(width))
}

func (p *ECRPage) syncImageTable() {
	p.imagePaginator.SetTotalPages(len(p.repositoryImages))
	if p.imagePaginator.Page >= p.imagePaginator.TotalPages {
		p.imagePaginator.Page = max(0, p.imagePaginator.TotalPages-1)
	}
	start, end := p.imagePaginator.GetSliceBounds(len(p.repositoryImages))
	p.imageTable.SetRows(ecrImageRowsForColumns(p.repositoryImages[start:end], p.imageTable.Columns()))
}

func (p *ECRPage) syncLocalTable() {
	filtered := p.filteredLocalImages()
	p.localPaginator.SetTotalPages(len(filtered))
	if p.localPaginator.Page >= p.localPaginator.TotalPages {
		p.localPaginator.Page = max(0, p.localPaginator.TotalPages-1)
	}
	if p.localIndex >= len(filtered) {
		p.localIndex = max(0, len(filtered)-1)
	}
	if len(filtered) > 0 {
		p.localPaginator.Page = p.localIndex / max(1, p.localPaginator.PerPage)
	}
	start, end := p.localPaginator.GetSliceBounds(len(filtered))
	if p.localIndex < start || p.localIndex >= end {
		p.localIndex = start
	}
	p.localTable.SetRows(ecrLocalImageRowsForColumns(filtered[start:end], p.localTable.Columns()))
	p.localTable.SetCursor(max(0, p.localIndex-start))
}

func ecrImageRowsForColumns(images []domainecr.RepositoryImage, cols []table.Column) []table.Row {
	rows := make([]table.Row, 0, len(images))
	for _, img := range images {
		tag := "<untagged>"
		if len(img.Tags) > 0 {
			tag = strings.Join(img.Tags, ",")
		}
		rows = append(rows, table.Row{
			tableutil.Truncate(tag, cols[0].Width),
			formatBytes(img.SizeBytes),
			formatTableTime(img.PushedAt),
			formatTableTime(img.LastRecordedPullAt),
			tableutil.Truncate(img.Digest, cols[4].Width),
		})
	}
	return rows
}

func ecrLocalImageRowsForColumns(images []domainecr.LocalImage, cols []table.Column) []table.Row {
	rows := make([]table.Row, 0, len(images))
	for _, img := range images {
		rows = append(rows, table.Row{
			tableutil.Truncate(img.Reference, cols[0].Width),
			formatBytes(img.SizeBytes),
			formatTableTime(img.CreatedAt),
			tableutil.Truncate(img.ID, cols[3].Width),
		})
	}
	return rows
}

func ecrImageRows(images []domainecr.RepositoryImage) []table.Row {
	return ecrImageRowsForColumns(images, ecrImageTableColumns())
}

func ecrLocalImageRows(images []domainecr.LocalImage) []table.Row {
	return ecrLocalImageRowsForColumns(images, ecrLocalImageTableColumns())
}

func formatBytes(bytes int64) string {
	if bytes < 0 {
		return "unknown"
	}
	const unit = 1000
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.2f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.2f PB", value/unit)
}

func formatTableTime(value time.Time) string {
	if value.IsZero() {
		return "never"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func truncateText(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 || len(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return value[:width-1] + "…"
}

func (p *ECRPage) filteredRepositories() []domainecr.Repository {
	query := strings.ToLower(strings.TrimSpace(p.searchInput.Value()))
	if query == "" {
		return p.repositories
	}
	out := make([]domainecr.Repository, 0, len(p.repositories))
	for _, repo := range p.repositories {
		if strings.Contains(strings.ToLower(repo.Name), query) || strings.Contains(strings.ToLower(repo.URI), query) {
			out = append(out, repo)
		}
	}
	return out
}

func (p *ECRPage) selectedSourceImage() string {
	filtered := p.filteredLocalImages()
	if len(filtered) > 0 && p.localIndex >= 0 && p.localIndex < len(filtered) {
		return filtered[p.localIndex].Reference
	}
	return strings.TrimSpace(p.manualInput.Value())
}

func (p *ECRPage) filteredLocalImages() []domainecr.LocalImage {
	query := strings.ToLower(strings.TrimSpace(p.manualInput.Value()))
	if query == "" {
		return p.localImages
	}
	filtered := make([]domainecr.LocalImage, 0, len(p.localImages))
	for _, image := range p.localImages {
		if strings.Contains(strings.ToLower(image.Reference), query) || strings.Contains(strings.ToLower(image.ID), query) {
			filtered = append(filtered, image)
		}
	}
	return filtered
}

func defaultTag(image string) string {
	image = strings.TrimSpace(image)
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash && lastColon < len(image)-1 {
		return image[lastColon+1:]
	}
	return "latest"
}

func (p *ECRPage) resetForSession() {
	p.cancelAll()
	p.loadedFor = ""
	p.loadingRepositories = false
	p.repositoryErr = ""
	p.repositories = nil
	p.repositoryIndex = 0
	p.selectedRepository = domainecr.Repository{}
	p.imagesLoading = false
	p.imagesErr = ""
	p.repositoryImages = nil
	p.localLoading = false
	p.localErr = ""
	p.localImages = nil
	p.localIndex = 0
	p.resetPushState()
	p.pushMessage = ""
	p.stage = ecrStageRepository
	p.searchInput.SetValue("")
	p.createInput.SetValue("")
}

func (p *ECRPage) resetPushState() {
	p.localLoading = false
	p.localErr = ""
	p.localImages = nil
	p.localIndex = 0
	p.manualInput.SetValue("")
	p.tagInput.SetValue("")
	p.plan = nil
	p.planErr = ""
	p.pushErr = ""
	p.pushProgress = domainecr.PushProgress{}
	p.pushResult = nil
	p.pushEvents = nil
	p.pushing = false
	p.planning = false
}

func (p *ECRPage) focusForStage() {
	p.searchInput.Blur()
	p.createInput.Blur()
	p.manualInput.Blur()
	p.tagInput.Blur()
	p.imageTable.Blur()
	p.localTable.Blur()
	switch p.stage {
	case ecrStageRepositoryImages:
		p.imageTable.Focus()
	case ecrStageLocalImage:
		p.localTable.Focus()
	case ecrStageCreateRepository:
		p.createInput.Focus()
	case ecrStageTag:
		p.tagInput.Focus()
	}
}
