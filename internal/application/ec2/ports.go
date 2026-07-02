package ec2

import (
	"context"

	domainec2 "aws-terminal/internal/domain/ec2"
)

type API interface {
	ListInstances(ctx context.Context, profileName, region string) ([]domainec2.Instance, error)
	StopInstance(ctx context.Context, input domainec2.StopInstanceInput) (domainec2.StopInstanceResult, error)
	TerminateInstance(ctx context.Context, input domainec2.TerminateInstanceInput) (domainec2.TerminateInstanceResult, error)
}
