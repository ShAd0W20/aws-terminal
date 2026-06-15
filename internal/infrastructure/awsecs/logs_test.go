package awsecs

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TestLogTargetsFromTaskDefinitionDerivesAwslogsStream(t *testing.T) {
	targets := logTargetsFromTaskDefinition([]ecstypes.ContainerDefinition{{
		Name: aws.String("app"),
		LogConfiguration: &ecstypes.LogConfiguration{
			LogDriver: ecstypes.LogDriverAwslogs,
			Options: map[string]string{
				"awslogs-group":         "/ecs/api",
				"awslogs-region":        "eu-west-1",
				"awslogs-stream-prefix": "ecs",
			},
		},
	}}, "abc123")
	if len(targets) != 1 {
		t.Fatalf("targets len = %d", len(targets))
	}
	got := targets[0]
	if !got.Supported {
		t.Fatalf("target should be supported: %+v", got)
	}
	if got.LogGroup != "/ecs/api" || got.Region != "eu-west-1" || got.StreamPrefix != "ecs" || got.TaskID != "abc123" || got.LogStream != "ecs/app/abc123" {
		t.Fatalf("unexpected target: %+v", got)
	}
}

func TestLogTargetsFromTaskDefinitionTreatsMissingPrefixAsUnsupported(t *testing.T) {
	targets := logTargetsFromTaskDefinition([]ecstypes.ContainerDefinition{{
		Name: aws.String("app"),
		LogConfiguration: &ecstypes.LogConfiguration{
			LogDriver: ecstypes.LogDriverAwslogs,
			Options:   map[string]string{"awslogs-group": "/ecs/api"},
		},
	}}, "abc123")
	if len(targets) != 1 {
		t.Fatalf("targets len = %d", len(targets))
	}
	if targets[0].Supported {
		t.Fatalf("missing stream prefix should be unsupported: %+v", targets[0])
	}
	if targets[0].Message == "" {
		t.Fatal("unsupported target should include friendly message")
	}
}

func TestAwslogsStreamNamePreservesLeadingSlashInPrefix(t *testing.T) {
	if got := awslogsStreamName("/ecs/backend", "/backend/", "/abc123/"); got != "/ecs/backend/backend/abc123" {
		t.Fatalf("stream = %q", got)
	}
}

func TestMatchingTaskLogStreamFindsActualStreamWhenGuessedStreamIsMissing(t *testing.T) {
	target := logTargetsFromTaskDefinition([]ecstypes.ContainerDefinition{{
		Name: aws.String("backend"),
		LogConfiguration: &ecstypes.LogConfiguration{
			LogDriver: ecstypes.LogDriverAwslogs,
			Options: map[string]string{
				"awslogs-group":         "/ecs/backend/backend-log-group",
				"awslogs-stream-prefix": "/ecs/backend",
			},
		},
	}}, "10c35930849f4b44892d2f89ab61aacf")[0]

	streams := []string{
		"/ecs/backend/backend/oldtask",
		"/ecs/backend/backend/10c35930849f4b44892d2f89ab61aacf",
		"/ecs/backend/other/10c35930849f4b44892d2f89ab61aacf",
	}
	if got := matchingTaskLogStream(streams, target); got != "/ecs/backend/backend/10c35930849f4b44892d2f89ab61aacf" {
		t.Fatalf("match = %q", got)
	}
}
