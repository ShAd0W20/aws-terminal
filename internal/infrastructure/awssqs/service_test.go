package awssqs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqssdk "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	appsqs "aws-terminal/internal/application/sqs"
	domainsqs "aws-terminal/internal/domain/sqs"
)

type fakeSQSClient struct {
	pages      []*awssqssdk.ListQueuesOutput
	attrs      map[string]map[string]string
	listCalls  int
	attrInputs []string
	received   bool
	purged     bool
}

func (f *fakeSQSClient) ListQueues(ctx context.Context, params *awssqssdk.ListQueuesInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.ListQueuesOutput, error) {
	if f.listCalls >= len(f.pages) {
		return nil, errors.New("unexpected list call")
	}
	page := f.pages[f.listCalls]
	f.listCalls++
	return page, nil
}

func (f *fakeSQSClient) GetQueueAttributes(ctx context.Context, params *awssqssdk.GetQueueAttributesInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.GetQueueAttributesOutput, error) {
	url := aws.ToString(params.QueueUrl)
	f.attrInputs = append(f.attrInputs, url)
	return &awssqssdk.GetQueueAttributesOutput{Attributes: f.attrs[url]}, nil
}
func (f *fakeSQSClient) ReceiveMessage(ctx context.Context, params *awssqssdk.ReceiveMessageInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.ReceiveMessageOutput, error) {
	f.received = true
	return &awssqssdk.ReceiveMessageOutput{Messages: []sqstypes.Message{{MessageId: aws.String("m1"), Body: aws.String("hello"), ReceiptHandle: aws.String("rh"), Attributes: map[string]string{string(sqstypes.MessageSystemAttributeNameSentTimestamp): "1000"}}}}, nil
}
func (f *fakeSQSClient) PurgeQueue(ctx context.Context, params *awssqssdk.PurgeQueueInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.PurgeQueueOutput, error) {
	f.purged = true
	return &awssqssdk.PurgeQueueOutput{}, nil
}

func TestListQueuesMapsAttributes(t *testing.T) {
	client := &fakeSQSClient{
		pages: []*awssqssdk.ListQueuesOutput{{QueueUrls: []string{"https://sqs.eu-west-1.amazonaws.com/123/orders"}}},
		attrs: map[string]map[string]string{
			"https://sqs.eu-west-1.amazonaws.com/123/orders": {
				string(sqstypes.QueueAttributeNameQueueArn):                              "arn:aws:sqs:eu-west-1:123:orders",
				string(sqstypes.QueueAttributeNameApproximateNumberOfMessages):           "42",
				string(sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible): "7",
			},
		},
	}
	svc := newServiceWithClient(client)
	queues, err := svc.ListQueues(context.Background(), "dev", "eu-west-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queues) != 1 {
		t.Fatalf("queues=%#v", queues)
	}
	queue := queues[0]
	if queue.Name != "orders" || queue.AvailableMessages != 42 || queue.InFlightMessages != 7 || queue.ARN == "" || queue.URL == "" {
		t.Fatalf("unexpected queue: %#v", queue)
	}
}

func TestListQueuesPaginates(t *testing.T) {
	client := &fakeSQSClient{
		pages: []*awssqssdk.ListQueuesOutput{
			{QueueUrls: []string{"https://sqs.us-east-1.amazonaws.com/123/a"}, NextToken: aws.String("next")},
			{QueueUrls: []string{"https://sqs.us-east-1.amazonaws.com/123/b"}},
		},
		attrs: map[string]map[string]string{
			"https://sqs.us-east-1.amazonaws.com/123/a": {},
			"https://sqs.us-east-1.amazonaws.com/123/b": {},
		},
	}
	queues, err := newServiceWithClient(client).ListQueues(context.Background(), "dev", "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queues) != 2 || client.listCalls != 2 {
		t.Fatalf("queues=%#v listCalls=%d", queues, client.listCalls)
	}
}

func TestReceiveMessagesAndPurgeQueue(t *testing.T) {
	client := &fakeSQSClient{}
	svc := newServiceWithClient(client)
	messages, err := svc.ReceiveMessages(context.Background(), appsqs.QueueActionInput{Profile: "dev", Region: "us-east-1", Queue: domainsqs.Queue{Name: "q", URL: "url"}, MaxCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].ID != "m1" || messages[0].Body != "hello" || messages[0].ReceiptHandle != "rh" || !messages[0].SentAt.Equal(time.Unix(1, 0)) {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	if err := svc.PurgeQueue(context.Background(), appsqs.QueueActionInput{Profile: "dev", Region: "us-east-1", Queue: domainsqs.Queue{Name: "q", URL: "url"}}); err != nil {
		t.Fatal(err)
	}
	if !client.received || !client.purged {
		t.Fatalf("received=%v purged=%v", client.received, client.purged)
	}
}

func TestQueueNameAndAttributeParsingFallbacks(t *testing.T) {
	if got := queueName("https://sqs.us-east-1.amazonaws.com/123/fallback", ""); got != "fallback" {
		t.Fatalf("queueName url fallback=%q", got)
	}
	if got := parseAttributeInt("not-a-number"); got != 0 {
		t.Fatalf("malformed count=%d", got)
	}
	if got := parseAttributeInt("-1"); got != 0 {
		t.Fatalf("negative count=%d", got)
	}
}
