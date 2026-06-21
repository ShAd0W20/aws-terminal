package sqs

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	appsqs "aws-terminal/internal/application/sqs"
	domainsqs "aws-terminal/internal/domain/sqs"
)

func (p *SQSPage) loadQueuesCmd(profile, region, sessionKey string) tea.Cmd {
	p.cancelLoad()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	return func() tea.Msg {
		queues, err := p.service.ListQueues(ctx, profile, region)
		return queuesLoadedMsg{sessionKey: sessionKey, queues: queues, err: err}
	}
}

func (p *SQSPage) receiveMessagesCmd(profile, region, sessionKey string, queue domainsqs.Queue) tea.Cmd {
	if p.messagesCancel != nil {
		p.messagesCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.messagesCancel = cancel
	return func() tea.Msg {
		messages, err := p.service.ReceiveMessages(ctx, appsqs.QueueActionInput{Profile: profile, Region: region, Queue: queue, MaxCount: 10})
		return messagesLoadedMsg{sessionKey: sessionKey, queueName: queue.Name, messages: messages, err: err}
	}
}

func (p *SQSPage) purgeQueueCmd(profile, region, sessionKey string, queue domainsqs.Queue) tea.Cmd {
	if p.purgeCancel != nil {
		p.purgeCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.purgeCancel = cancel
	return func() tea.Msg {
		err := p.service.PurgeQueue(ctx, appsqs.QueueActionInput{Profile: profile, Region: region, Queue: queue})
		return queuePurgedMsg{sessionKey: sessionKey, queueName: queue.Name, err: err}
	}
}
