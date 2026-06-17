package awsecs

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TestTaskDefinitionSummaryFromARNParsesFamilyRevision(t *testing.T) {
	summary := taskDefinitionSummaryFromARN("arn:aws:ecs:eu-west-1:123:task-definition/api:42", "ACTIVE")
	if summary.ARN != "arn:aws:ecs:eu-west-1:123:task-definition/api:42" || summary.DisplayName != "api:42" || summary.Family != "api" || summary.Revision != 42 || summary.Status != "ACTIVE" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestTaskFromSDKMapsStoppedTaskFields(t *testing.T) {
	task := taskFromSDK(ecstypes.Task{
		TaskArn:       aws.String("arn:aws:ecs:eu-west-1:123:task/backend/abc123"),
		LastStatus:    aws.String("STOPPING"),
		DesiredStatus: aws.String("STOPPED"),
		StoppedReason: aws.String("Stopped from aws-terminal"),
	})
	if task.ID != "abc123" || task.LastStatus != "STOPPING" || task.DesiredStatus != "STOPPED" || task.StoppedReason != "Stopped from aws-terminal" {
		t.Fatalf("unexpected stopped task mapping: %#v", task)
	}
}

func TestTaskFromSDKUsesContainerNetworkInterfacePrivateIP(t *testing.T) {
	task := taskFromSDK(ecstypes.Task{
		TaskArn: aws.String("arn:aws:ecs:eu-west-1:123:task/backend/abc123"),
		Containers: []ecstypes.Container{{
			Name: aws.String("app"),
			NetworkInterfaces: []ecstypes.NetworkInterface{{
				PrivateIpv4Address: aws.String("10.0.18.21"),
			}},
		}},
	})

	if task.PrivateIP != "10.0.18.21" {
		t.Fatalf("PrivateIP = %q, want 10.0.18.21", task.PrivateIP)
	}
}

func TestTaskFromSDKFallsBackToAttachmentPrivateIP(t *testing.T) {
	task := taskFromSDK(ecstypes.Task{
		TaskArn: aws.String("arn:aws:ecs:eu-west-1:123:task/backend/abc123"),
		Attachments: []ecstypes.Attachment{{
			Details: []ecstypes.KeyValuePair{{
				Name:  aws.String("privateIPv4Address"),
				Value: aws.String("10.0.19.68"),
			}},
		}},
	})

	if task.PrivateIP != "10.0.19.68" {
		t.Fatalf("PrivateIP = %q, want 10.0.19.68", task.PrivateIP)
	}
}
