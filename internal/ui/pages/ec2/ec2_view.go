package ec2

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
	"aws-terminal/internal/ui/workflow"
)

func (p *EC2Page) View(state pageapi.State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{styles.SectionTitleStyle.Render("EC2"), styles.SubtitleStyle.Render("Browse and manage EC2 instances."), ""}
	if state.ActiveSession == nil {
		lines = append(lines, styles.MutedStyle.Render("No active AWS session. Authenticate a profile first."))
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
		lines = append(lines, styles.MutedStyle.Render("Focus Page to interact with EC2."))
	}
	if p.actionMessage != "" {
		lines = append(lines, styles.StatusStyle.Render(p.actionMessage))
	}
	lines = append(lines, "")

	switch p.stage {
	case ec2StageInstances:
		lines = append(lines, p.instanceLines(width, height, len(lines))...)
	case ec2StageInstanceDetail:
		lines = append(lines, p.instanceDetailLines()...)
	case ec2StageStopReview:
		lines = append(lines, p.stopReviewLines()...)
	case ec2StageStopping:
		lines = append(lines, p.stoppingLines()...)
	case ec2StageTerminateConfirm:
		lines = append(lines, p.terminateConfirmLines()...)
	case ec2StageTerminating:
		lines = append(lines, p.terminatingLines()...)
	}
	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}

func (p *EC2Page) ShortHelp() []key.Binding {
	switch p.stage {
	case ec2StageInstances:
		return []key.Binding{ec2UpKey, ec2DownKey, ec2EnterKey, ec2SearchKey, ec2RefreshKey, ec2CancelKey, ec2TabKey}
	case ec2StageInstanceDetail:
		keys := []key.Binding{ec2RefreshKey}
		if isStoppable(p.selected) {
			keys = append(keys, ec2StopKey)
		}
		if isTerminable(p.selected) {
			keys = append(keys, ec2TerminateKey)
		}
		return append(keys, ec2BackKey, ec2CancelKey, ec2TabKey)
	case ec2StageStopReview, ec2StageTerminateConfirm:
		return []key.Binding{ec2EnterKey, ec2BackKey, ec2CancelKey, ec2TabKey}
	case ec2StageStopping, ec2StageTerminating:
		return []key.Binding{ec2CancelKey, ec2TabKey}
	default:
		return []key.Binding{ec2TabKey}
	}
}

func (p *EC2Page) FullHelp() [][]key.Binding { return [][]key.Binding{p.ShortHelp()} }

func (p *EC2Page) instanceLines(width, height, usedLines int) []string {
	searchHint := "Ctrl+F search · r refresh · Enter details"
	if p.search.Focused() {
		searchHint = "Search active. Type to filter; Esc leaves search."
	}
	lines := []string{styles.MutedStyle.Render("Instances · terminated instances are hidden"), styles.MutedStyle.Render(searchHint), p.search.View()}
	if p.loading {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading instances..."))
	}
	if p.loadErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.loadErr))
	}
	if p.actionErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.actionErr))
	}
	instances := p.filteredInstances()
	if len(instances) == 0 {
		if p.loading {
			return lines
		}
		if strings.TrimSpace(p.search.Value()) != "" {
			return append(lines, styles.MutedStyle.Render("No EC2 instances match your search."))
		}
		return append(lines, styles.MutedStyle.Render("No non-terminated EC2 instances found in this region."))
	}
	tableWidth := max(72, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	p.configureTable(tableWidth, height-usedLines-len(lines)-6)
	p.syncTable()
	lines = append(lines, "", tableutil.RenderBox(p.table.View(), tableWidth+4))
	selected := p.currentInstance()
	if selected.ID != "" {
		lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Selected %s · %s · %s", selected.ID, firstValue(selected.Name, "unnamed"), firstValue(selected.State, "unknown"))))
	}
	return lines
}

func (p *EC2Page) instanceDetailLines() []string {
	i := p.selected
	lines := []string{
		styles.MutedStyle.Render("Instance detail"),
		"",
		fmt.Sprintf("%s  •  %s  •  %s", statusLabel(i.State), firstValue(i.Type, "unknown type"), firstValue(i.AvailabilityZone, "unknown AZ")),
	}
	if p.actionErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.actionErr))
	}
	lines = append(lines,
		"",
		styles.MutedStyle.Render("Network"),
		detailKV("Private IP", i.PrivateIP),
		detailKV("Public IP", i.PublicIP),
		detailKV("VPC", i.VpcID),
		detailKV("Subnet", i.SubnetID),
		detailKV("Private DNS", i.PrivateDNS),
		detailKV("Public DNS", i.PublicDNS),
		"",
		styles.MutedStyle.Render("Runtime"),
		detailKV("AMI", i.ImageID),
		detailKV("Architecture", i.Architecture),
		detailKV("Platform", i.Platform),
		detailKV("Key pair", i.KeyName),
		detailKV("Launched", formatTime(i.LaunchTime)),
		detailKV("IAM profile", i.IAMInstanceProfile),
		"",
		styles.MutedStyle.Render("Security"),
		detailKV("Security groups", securityGroupText(i.SecurityGroups)),
	)
	if len(i.BlockDevices) > 0 {
		lines = append(lines, "", styles.MutedStyle.Render("Block devices"))
		for _, block := range i.BlockDevices {
			lines = append(lines, fmt.Sprintf("%s  •  %s  •  delete on termination: %t", firstValue(block.DeviceName, "-"), firstValue(block.VolumeID, "-"), block.DeleteOnTermination))
		}
	}
	if len(i.NetworkInterfaces) > 0 {
		lines = append(lines, "", styles.MutedStyle.Render("Network interfaces"))
		for _, networkInterface := range i.NetworkInterfaces {
			lines = append(lines, fmt.Sprintf("%s  •  %s  •  private %s  •  public %s", firstValue(networkInterface.ID, "-"), firstValue(networkInterface.Status, "-"), firstValue(networkInterface.PrivateIP, "-"), firstValue(networkInterface.PublicIP, "-")))
		}
	}
	if len(i.Tags) > 0 {
		lines = append(lines, "", styles.MutedStyle.Render("Tags"))
		limit := len(i.Tags)
		if limit > 8 {
			limit = 8
		}
		for _, tag := range i.Tags[:limit] {
			lines = append(lines, detailKV(tag.Key, tag.Value))
		}
		if len(i.Tags) > limit {
			lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("... %d more tags", len(i.Tags)-limit)))
		}
	}
	lines = append(lines,
		"",
		styles.MutedStyle.Render("Identifiers"),
		detailKV("Name", i.Name),
		detailKV("Instance ID", i.ID),
		"",
		styles.MutedStyle.Render("s stops running instances · x terminates with typed confirmation · b/Esc returns"),
	)
	return lines
}

func (p *EC2Page) stopReviewLines() []string {
	lines := []string{
		styles.MutedStyle.Render("Stop instance"),
		fmt.Sprintf("Instance: %s", p.selected.ID),
		fmt.Sprintf("Name: %s", firstValue(p.selected.Name, "-")),
		fmt.Sprintf("Current state: %s", firstValue(p.selected.State, "unknown")),
		"",
		styles.MutedStyle.Render("Stopping is reversible. The instance will shut down and can be started later."),
		styles.MutedStyle.Render("Press Enter to request stop, or b/Esc to cancel."),
	}
	if p.actionErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.actionErr))
	}
	return lines
}

func (p *EC2Page) stoppingLines() []string {
	return []string{styles.MutedStyle.Render("Stop instance"), fmt.Sprintf("Instance: %s", p.selected.ID), "", styles.StatusStyle.Render(p.spinner.View() + " Requesting instance stop..."), styles.MutedStyle.Render("Esc cancels waiting; AWS may still process the request if it already reached EC2.")}
}

func (p *EC2Page) terminateConfirmLines() []string {
	lines := []string{
		styles.ErrorStyle.Render("Terminate instance"),
		fmt.Sprintf("Instance: %s", p.selected.ID),
		fmt.Sprintf("Name: %s", firstValue(p.selected.Name, "-")),
		fmt.Sprintf("Current state: %s", firstValue(p.selected.State, "unknown")),
		"",
		styles.ErrorStyle.Render("This permanently terminates the instance."),
		styles.MutedStyle.Render("Instance-store data will be lost. EBS volumes may be deleted depending on DeleteOnTermination."),
		"",
		styles.MutedStyle.Render("Type the instance ID exactly, then press Enter:"),
		p.terminateInput.View(),
	}
	if p.actionErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.actionErr))
	}
	lines = append(lines, "", styles.MutedStyle.Render("b/Esc cancels."))
	return lines
}

func (p *EC2Page) terminatingLines() []string {
	return []string{styles.ErrorStyle.Render("Terminate instance"), fmt.Sprintf("Instance: %s", p.selected.ID), "", styles.StatusStyle.Render(p.spinner.View() + " Requesting instance termination..."), styles.MutedStyle.Render("Esc cancels waiting; AWS may still process the request if it already reached EC2.")}
}
