package ec2

import (
	"context"
	"os/exec"

	"github.com/charmbracelet/bubbletea"

	domainec2 "aws-terminal/internal/domain/ec2"
)

func (p *EC2Page) loadInstancesCmd(profile, region, sessionKey string) tea.Cmd {
	p.cancelLoad()
	ctx, cancel := context.WithCancel(context.Background())
	p.loadCancel = cancel
	return func() tea.Msg {
		instances, err := p.service.ListInstances(ctx, profile, region)
		return instancesLoadedMsg{sessionKey: sessionKey, instances: instances, err: err}
	}
}

func (p *EC2Page) stopInstanceCmd(input domainec2.StopInstanceInput, sessionKey string) tea.Cmd {
	p.cancelAction()
	ctx, cancel := context.WithCancel(context.Background())
	p.actionCancel = cancel
	return func() tea.Msg {
		result, err := p.service.StopInstance(ctx, input)
		return instanceStoppedMsg{sessionKey: sessionKey, instanceID: input.InstanceID, result: result, err: err}
	}
}

func (p *EC2Page) terminateInstanceCmd(input domainec2.TerminateInstanceInput, sessionKey string) tea.Cmd {
	p.cancelAction()
	ctx, cancel := context.WithCancel(context.Background())
	p.actionCancel = cancel
	return func() tea.Msg {
		result, err := p.service.TerminateInstance(ctx, input)
		return instanceTerminatedMsg{sessionKey: sessionKey, instanceID: input.InstanceID, result: result, err: err}
	}
}

func (p *EC2Page) connectInstanceCmd(profile, region, instanceID, sessionKey string) tea.Cmd {
	cmd := buildSessionManagerCommand(profile, region, instanceID)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return instanceConnectionFinishedMsg{sessionKey: sessionKey, instanceID: instanceID, err: err}
	})
}

func (p *EC2Page) cancelLoad() {
	if p.loadCancel != nil {
		p.loadCancel()
		p.loadCancel = nil
	}
}

func (p *EC2Page) cancelAction() {
	if p.actionCancel != nil {
		p.actionCancel()
		p.actionCancel = nil
	}
}

func (p *EC2Page) cancelAll() {
	p.cancelLoad()
	p.cancelAction()
}

func buildSessionManagerCommand(profile, region, instanceID string) *exec.Cmd {
	return exec.Command("aws", "ssm", "start-session", "--target", instanceID, "--profile", profile, "--region", region)
}
