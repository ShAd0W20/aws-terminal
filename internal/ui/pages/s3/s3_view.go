package s3

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"

	domains3 "aws-terminal/internal/domain/s3"
	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
)

func (p *S3Page) View(state pageapi.State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := []string{
		styles.SectionTitleStyle.Render("S3 Sync"),
		styles.SubtitleStyle.Render("List buckets, browse a local file or folder, review the sync plan, then execute it."),
		"",
	}

	if state.ActiveSession == nil {
		lines = append(lines,
			styles.MutedStyle.Render("No active AWS profile. Authenticate a profile from the sidebar first."),
			styles.MutedStyle.Render("After authentication, the S3 page will load all accessible buckets for the active profile."),
		)
		return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
	}

	lines = append(lines,
		fmt.Sprintf("Active profile: %s", state.ActiveSession.Profile),
		fmt.Sprintf("Account: %s", workflow.ValueOrFallback(state.ActiveSession.Account, "unknown")),
		fmt.Sprintf("Region: %s", workflow.ValueOrFallback(workflow.ActiveRegion(state), "unknown")),
	)

	if state.PageFocused {
		lines = append(lines, styles.StatusStyle.Render("Page focus active · tab returns to navigation."))
	} else {
		lines = append(lines, styles.MutedStyle.Render("Focus Page to interact with S3."))
	}

	lines = append(lines, "")
	lines = append(lines, p.workflowSummaryLines()...)
	lines = append(lines, "")

	switch p.stage {
	case s3StageBucket:
		lines = append(lines, p.bucketStageLines(width, height)...)
	case s3StageSource:
		lines = append(lines, p.sourceStageLines(width, height)...)
	case s3StagePrefix:
		lines = append(lines, p.prefixStageLines(width)...)
	case s3StageReview:
		reviewHeight := max(4, height-len(lines)-styles.PanelStyle.GetVerticalFrameSize()-3)
		lines = append(lines, p.reviewStageLines(width, reviewHeight)...)
	case s3StageConfirmDelete:
		lines = append(lines, p.confirmDeleteStageLines(width)...)
	case s3StageSync:
		lines = append(lines, p.syncStageLines(width)...)
	}

	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}

func (p *S3Page) ShortHelp() []key.Binding {
	switch p.stage {
	case s3StageBucket:
		if p.offerInvalidation {
			return []key.Binding{s3BucketUpKey, s3BucketDownKey, s3InvalidateKey, s3EnterKey, s3CancelKey, s3TabHelpKey}
		}
		return []key.Binding{s3BucketUpKey, s3BucketDownKey, s3EnterKey, s3CancelKey, s3TabHelpKey}
	case s3StageSource:
		return []key.Binding{s3PickerOpenKey, s3PickerSelectKey, s3PickerBackKey, s3BackKey, s3CancelKey}
	case s3StagePrefix:
		return []key.Binding{s3EnterKey, s3BackKey, s3CancelKey, s3TabHelpKey}
	case s3StageReview:
		return []key.Binding{s3ViewportUpKey, s3ViewportDownKey, s3ToggleKey, s3OptimizeKey, s3MetadataKey, s3EnterKey, s3BackKey, s3CancelKey}
	case s3StageConfirmDelete:
		return []key.Binding{s3EnterKey, s3BackKey, s3CancelKey, s3TabHelpKey}
	case s3StageSync:
		return []key.Binding{s3BackKey, s3CancelKey, s3TabHelpKey}
	default:
		return []key.Binding{s3TabHelpKey}
	}
}

func (p *S3Page) FullHelp() [][]key.Binding {
	switch p.stage {
	case s3StageBucket:
		if p.offerInvalidation {
			return [][]key.Binding{{s3BucketUpKey, s3BucketDownKey, s3InvalidateKey, s3EnterKey, s3CancelKey}, {s3TabHelpKey}}
		}
		return [][]key.Binding{{s3BucketUpKey, s3BucketDownKey, s3EnterKey, s3CancelKey}, {s3TabHelpKey}}
	case s3StageSource:
		return [][]key.Binding{{s3PickerOpenKey, s3PickerSelectKey, s3PickerBackKey}, {s3BackKey, s3CancelKey, s3TabHelpKey}}
	case s3StagePrefix:
		return [][]key.Binding{{s3EnterKey, s3BackKey, s3CancelKey}, {s3TabHelpKey}}
	case s3StageReview:
		return [][]key.Binding{{s3ViewportUpKey, s3ViewportDownKey, s3PageUpKey, s3PageDownKey}, {s3ToggleKey, s3OptimizeKey, s3MetadataKey, s3EnterKey, s3BackKey, s3CancelKey, s3TabHelpKey}}
	case s3StageConfirmDelete:
		return [][]key.Binding{{s3EnterKey, s3BackKey, s3CancelKey}, {s3TabHelpKey}}
	case s3StageSync:
		return [][]key.Binding{{s3BackKey, s3CancelKey}, {s3TabHelpKey}}
	default:
		return [][]key.Binding{{s3TabHelpKey}}
	}
}

func (p *S3Page) workflowSummaryLines() []string {
	source := "not selected"
	if p.sourceInfo != nil {
		source = p.sourceInfo.Path
	}

	prefix := strings.TrimSpace(p.prefixInput.Value())
	if prefix == "" {
		prefix = "bucket root"
	}

	return []string{
		styles.MutedStyle.Render("Workflow summary"),
		fmt.Sprintf("Bucket: %s", workflow.ValueOrFallback(p.selectedBucket, "not selected")),
		fmt.Sprintf("Source: %s", source),
		fmt.Sprintf("Prefix: %s", prefix),
	}
}

func (p *S3Page) bucketStageLines(width, height int) []string {
	lines := []string{styles.MutedStyle.Render("Step 1 of 4 · Select a destination bucket")}
	if p.loadingBuckets {
		lines = append(lines, styles.StatusStyle.Render("Loading buckets for the active profile..."))
		return lines
	}
	if p.syncMessage != "" {
		lines = append(lines, styles.StatusStyle.Render(p.syncMessage), "")
	}
	if p.bucketErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.bucketErr))
	}
	if len(p.buckets) == 0 {
		lines = append(lines, styles.MutedStyle.Render("No buckets were returned for this profile."))
		return lines
	}

	visible := max(5, height-18)
	start := max(0, p.bucketIndex-visible/2)
	end := min(len(p.buckets), start+visible)
	if end-start < visible {
		start = max(0, end-visible)
	}

	listLines := []string{}
	listWidth := max(32, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	for index := start; index < end; index++ {
		prefix := "  "
		style := styles.SidebarItemStyle
		if index == p.bucketIndex {
			prefix = "▸ "
			style = styles.FocusedSelectedSidebarItemStyle
		}
		bucket := p.buckets[index]
		nameWidth := max(12, listWidth-24)
		line := fmt.Sprintf("%s%s", prefix, tableutil.Truncate(bucket.Name, nameWidth))
		if !bucket.CreationDate.IsZero() {
			line += styles.MutedStyle.Render("  created " + bucket.CreationDate.Local().Format("2006-01-02"))
		}
		listLines = append(listLines, style.Render(line))
	}
	lines = append(lines, tableutil.RenderBox(strings.Join(listLines, "\n"), listWidth+4))

	lines = append(lines, "", styles.MutedStyle.Render("Enter selects bucket · keys in footer"))
	return lines
}

func (p *S3Page) sourceStageLines(width, height int) []string {
	lines := []string{
		styles.MutedStyle.Render("Step 2 of 4 · Choose a local file or folder"),
		fmt.Sprintf("Destination bucket: %s", workflow.ValueOrFallback(p.selectedBucket, "not selected")),
		fmt.Sprintf("Current directory: %s", p.picker.CurrentDirectory),
	}
	if p.sourceErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.sourceErr))
	}
	if p.inspectingSource {
		lines = append(lines, styles.StatusStyle.Render("Inspecting the selected local path..."))
	}
	if p.sourceInfo != nil {
		lines = append(lines,
			styles.StatusStyle.Render(fmt.Sprintf("Selected source: %s (%d files, %s)", p.sourceInfo.Path, p.sourceInfo.FileCount(), formatBytes(p.sourceInfo.TotalSize))),
		)
	}

	lines = append(lines, "")
	pickerHeight := max(8, height/3)
	p.picker.SetHeight(pickerHeight)
	lines = append(lines, p.picker.View())
	lines = append(lines, "", styles.MutedStyle.Render("Open/select with footer keys · b returns"))
	return lines
}

func (p *S3Page) prefixStageLines(width int) []string {
	lines := []string{
		styles.MutedStyle.Render("Step 3 of 4 · Optional destination prefix"),
		fmt.Sprintf("Selected bucket: %s", workflow.ValueOrFallback(p.selectedBucket, "not selected")),
	}
	if p.sourceInfo != nil {
		lines = append(lines, fmt.Sprintf("Selected source: %s (%d files, %s)", p.sourceInfo.Path, p.sourceInfo.FileCount(), formatBytes(p.sourceInfo.TotalSize)))
	}
	p.prefixInput.Width = max(10, width-12)
	lines = append(lines, "", p.prefixInput.View())
	lines = append(lines, "", styles.MutedStyle.Render("Empty prefix uses bucket root · Enter builds plan"))
	return lines
}

func (p *S3Page) reviewStageLines(width, height int) []string {
	lines := []string{styles.MutedStyle.Render("Step 4 of 4 · Review and confirm")}
	if p.sourceInfo == nil {
		lines = append(lines, styles.ErrorStyle.Render("No source is selected."))
		return lines
	}

	destination := "s3://" + p.selectedBucket + "/"
	if prefix := strings.Trim(strings.TrimSpace(p.prefixInput.Value()), "/"); prefix != "" {
		destination += prefix + "/"
	}
	deleteLabel := "off"
	if p.deleteEnabled {
		deleteLabel = "on"
	}
	planningLabel := "full refresh (upload every local file)"
	planningTradeoff := "Safest mode: refreshes content and metadata even when local and remote sizes match."
	if p.optimizedPlanning {
		planningLabel = "size-only optimized (skip same-size remote objects)"
		planningTradeoff = "Faster/cheaper repeat syncs, but same-size content or metadata changes can be missed."
	}

	metadataLabel := "off"
	if p.staticWebsitePreset {
		metadataLabel = "static website (HTML no-cache, hashed assets immutable, .gz/.br encoding)"
	}

	lines = append(lines,
		fmt.Sprintf("Source path: %s", p.sourceInfo.Path),
		fmt.Sprintf("Source kind: %s", p.sourceInfo.Kind),
		fmt.Sprintf("Destination: %s", destination),
		fmt.Sprintf("Delete extra remote files: %s", deleteLabel),
		fmt.Sprintf("Upload planning: %s", planningLabel),
		styles.MutedStyle.Render(planningTradeoff),
		fmt.Sprintf("Metadata preset: %s", metadataLabel),
	)

	if p.sourceInfo.Kind == domains3.SourceKindFile {
		lines = append(lines, styles.MutedStyle.Render("Delete is automatically disabled when the source is a single file."))
	}
	if p.planning {
		lines = append(lines, "", styles.StatusStyle.Render("Building the sync plan against S3..."))
		return lines
	}
	if p.planErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.planErr))
		return lines
	}
	if p.plan == nil {
		lines = append(lines, "", styles.MutedStyle.Render("No sync plan is available yet."))
		return lines
	}

	lines = append(lines,
		"",
		styles.MutedStyle.Render("Planned actions"),
		fmt.Sprintf("Uploads: %d", p.plan.UploadCount()),
		fmt.Sprintf("Unchanged / skipped: %d", p.plan.SkipCount()),
		fmt.Sprintf("Deletes: %d", p.plan.DeleteCount()),
	)

	appendObjects := func(title string, count int, keyAt func(int) string) {
		lines = append(lines, "", styles.MutedStyle.Render(title))
		if count == 0 {
			lines = append(lines, styles.MutedStyle.Render("• none"))
			return
		}

		for i := 0; i < count; i++ {
			lines = append(lines, "• "+keyAt(i))
		}
	}

	appendObjects("Uploads", len(p.plan.Uploads), func(i int) string { return p.plan.Uploads[i].Key })
	appendObjects("Deletes", len(p.plan.Deletes), func(i int) string { return p.plan.Deletes[i].Key })
	appendObjects("Skipped", len(p.plan.Skips), func(i int) string { return p.plan.Skips[i].Key })
	lines = append(lines,
		"",
		styles.MutedStyle.Render("Review actions · keys in footer"),
	)

	content := strings.Join(lines, "\n")
	p.reviewViewport.Width = max(1, width-6)
	p.reviewViewport.Height = max(3, height)
	p.reviewViewport.SetContent(content)

	viewportLines := []string{p.reviewViewport.View()}
	if strings.Count(content, "\n")+1 > p.reviewViewport.Height {
		viewportLines = append(viewportLines, styles.MutedStyle.Render(fmt.Sprintf("Scroll: %.0f%%", p.reviewViewport.ScrollPercent()*100)))
	}
	return viewportLines
}

func (p *S3Page) confirmDeleteStageLines(width int) []string {
	deleteCount := 0
	if p.plan != nil {
		deleteCount = p.plan.DeleteCount()
	}
	p.confirmInput.Width = max(10, width-20)

	lines := []string{
		styles.ErrorStyle.Render("Destructive action confirmation required"),
		fmt.Sprintf("This sync will delete %d remote S3 objects from s3://%s/%s.", deleteCount, p.selectedBucket, strings.Trim(strings.TrimSpace(p.prefixInput.Value()), "/")),
		styles.MutedStyle.Render("Type DELETE exactly · Enter confirms · b/Esc returns"),
		"",
		p.confirmInput.View(),
	}
	if strings.TrimSpace(p.confirmInput.Value()) != "" && !deleteConfirmationMatches(p.confirmInput.Value()) {
		lines = append(lines, "", styles.ErrorStyle.Render("Confirmation does not match DELETE."))
	}
	return lines
}

func (p *S3Page) syncStageLines(width int) []string {
	lines := []string{styles.MutedStyle.Render("Sync execution")}
	compact := width < 72
	p.syncBar.ShowPercentage = !compact
	if compact {
		p.syncBar.Width = max(8, min(20, width-10))
	} else {
		p.syncBar.Width = min(48, max(12, width-18))
	}

	if p.syncing {
		phase := "Syncing objects"
		switch p.syncProgress.Stage {
		case "uploading":
			phase = "Uploading files"
		case "deleting":
			phase = "Deleting remote files"
		}

		lines = append(lines,
			styles.StatusStyle.Render(phase),
			p.syncBar.ViewAs(syncPercent(p.syncProgress)),
		)

		meta := fmt.Sprintf("Completed: %d/%d", p.syncProgress.Current, p.syncProgress.Total)
		if p.syncProgress.TotalUploadBytes > 0 {
			meta += fmt.Sprintf(" • Uploaded bytes: %s/%s", formatBytes(p.syncProgress.UploadedBytes), formatBytes(p.syncProgress.TotalUploadBytes))
		}
		if !p.lastSyncStartedAt.IsZero() {
			if p.syncProgress.TotalUploadBytes > 0 && p.syncProgress.UploadedBytes > 0 && p.syncProgress.UploadedBytes < p.syncProgress.TotalUploadBytes {
				elapsed := time.Since(p.lastSyncStartedAt)
				remaining := time.Duration(float64(elapsed) * float64(p.syncProgress.TotalUploadBytes-p.syncProgress.UploadedBytes) / float64(p.syncProgress.UploadedBytes))
				if remaining > 0 {
					meta += " • ETA: " + remaining.Round(time.Second).String()
				}
			} else if p.syncProgress.Current > 0 && p.syncProgress.Current < p.syncProgress.Total {
				elapsed := time.Since(p.lastSyncStartedAt)
				remaining := time.Duration(float64(elapsed) * float64(p.syncProgress.Total-p.syncProgress.Current) / float64(p.syncProgress.Current))
				if remaining > 0 {
					meta += " • ETA: " + remaining.Round(time.Second).String()
				}
			}
		}
		lines = append(lines, meta)

		if strings.TrimSpace(p.syncProgress.Detail) != "" {
			label := "Latest item"
			if compact {
				label = "Item"
			}
			lines = append(lines, fmt.Sprintf("%s: %s", label, p.syncProgress.Detail))
		}
		if compact {
			lines = append(lines, fmt.Sprintf("U:%d  D:%d  S:%d", p.syncProgress.Uploaded, p.syncProgress.Deleted, p.syncProgress.Skipped))
		} else {
			lines = append(lines,
				fmt.Sprintf("Uploaded: %d", p.syncProgress.Uploaded),
				fmt.Sprintf("Deleted: %d", p.syncProgress.Deleted),
				fmt.Sprintf("Skipped: %d", p.syncProgress.Skipped),
			)
		}
		return lines
	}

	if p.syncErr != "" {
		phase := "Sync failed"
		switch p.syncProgress.Stage {
		case "uploading":
			phase = "Upload failed"
		case "deleting":
			phase = "Delete phase failed"
		}

		lines = append(lines,
			styles.ErrorStyle.Render(phase+"."),
			p.syncBar.ViewAs(syncPercent(p.syncProgress)),
		)
		if p.syncResult != nil {
			if compact {
				lines = append(lines, fmt.Sprintf("U:%d  D:%d  S:%d", p.syncResult.Uploaded, p.syncResult.Deleted, p.syncResult.Skipped))
			} else {
				lines = append(lines,
					fmt.Sprintf("Uploaded before failure: %d", p.syncResult.Uploaded),
					fmt.Sprintf("Deleted before failure: %d", p.syncResult.Deleted),
					fmt.Sprintf("Skipped: %d", p.syncResult.Skipped),
				)
			}
		}
		lines = append(lines, styles.MutedStyle.Render(p.syncErr))
	}
	if !p.lastSyncStartedAt.IsZero() {
		duration := p.lastSyncFinishedAt.Sub(p.lastSyncStartedAt)
		if duration > 0 {
			lines = append(lines, fmt.Sprintf("Duration: %s", duration.Round(time.Second)))
		}
	}
	if p.syncing {
		lines = append(lines, "", styles.MutedStyle.Render("Esc cancels running sync"))
	} else {
		lines = append(lines, "", styles.MutedStyle.Render("b/Esc returns to review"))
	}
	return lines
}
