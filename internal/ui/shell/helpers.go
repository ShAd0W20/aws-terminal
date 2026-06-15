package shell

import "aws-terminal/internal/ui/styles"

const (
	preferredProfilePaneHeight = 6
	preferredRegionPaneHeight  = 8
	preferredPagePaneHeight    = 5
	minSidebarPaneHeight       = 1
)

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func clamp(value, minValue, maxValue int) int {
	return min(max(value, minValue), maxValue)
}

func sidebarDimensions(totalWidth, totalHeight int) (int, int) {
	if totalWidth <= 0 || totalHeight <= 0 {
		return 0, 0
	}

	if totalWidth < 72 {
		return totalWidth, min(max(12, totalHeight/3), max(12, totalHeight/2))
	}

	if totalWidth < 160 || totalHeight < 42 {
		return clamp(totalWidth/4, 24, 34), totalHeight
	}

	return clamp(totalWidth/3, 28, 38), totalHeight
}

func shouldCollapseSidebar(totalWidth, totalHeight int, pageFocused bool) bool {
	return pageFocused && (totalWidth < 160 || totalHeight < 44)
}

func sidebarPaneHeights(height, totalProfiles, totalRegions, totalPages int) (profileHeight, regionHeight, pageHeight int, showHint bool) {
	contentHeight := max(1, height-styles.SidebarPanelStyle.GetVerticalFrameSize())
	showHint = contentHeight >= 22

	sectionCount := 3
	reservedLines := sectionCount + (sectionCount - 1)
	if showHint {
		reservedLines += 2
	}

	available := max(sectionCount, contentHeight-reservedLines)
	heights := []int{1, 1, 1}
	remaining := max(0, available-sectionCount)

	profileTarget := min(max(1, totalProfiles), preferredProfilePaneHeight)
	regionTarget := min(max(1, totalRegions), preferredRegionPaneHeight)
	pageTarget := min(max(1, totalPages), preferredPagePaneHeight)

	heights[0], remaining = growPaneHeight(heights[0], profileTarget, remaining)
	heights[1], remaining = growPaneHeight(heights[1], regionTarget, remaining)
	heights[2], remaining = growPaneHeight(heights[2], pageTarget, remaining)

	// When there is extra vertical room, favor the Regions pane first since it
	// usually has the longest catalog and benefits the most from extra rows.
	heights[1], remaining = growPaneHeight(heights[1], max(1, totalRegions), remaining)
	heights[0], remaining = growPaneHeight(heights[0], max(1, totalProfiles), remaining)
	heights[2], remaining = growPaneHeight(heights[2], max(1, totalPages), remaining)

	if remaining > 0 {
		heights[1] += remaining
	}

	return heights[0], heights[1], heights[2], showHint
}

func growPaneHeight(current, target, remaining int) (int, int) {
	if target <= current || remaining <= 0 {
		return current, remaining
	}

	growth := min(target-current, remaining)
	return current + growth, remaining - growth
}

func sidebarContentWidth(width int) int {
	return max(1, width-styles.SidebarPanelStyle.GetHorizontalFrameSize())
}
