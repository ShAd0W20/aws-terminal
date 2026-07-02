package awsec2

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	domainec2 "aws-terminal/internal/domain/ec2"
)

type fakeEC2Client struct {
	describePages []*awsec2sdk.DescribeInstancesOutput
	describeCalls int
	stopIDs       []string
	terminateIDs  []string
}

func (f *fakeEC2Client) DescribeInstances(ctx context.Context, params *awsec2sdk.DescribeInstancesInput, optFns ...func(*awsec2sdk.Options)) (*awsec2sdk.DescribeInstancesOutput, error) {
	if f.describeCalls >= len(f.describePages) {
		return &awsec2sdk.DescribeInstancesOutput{}, nil
	}
	page := f.describePages[f.describeCalls]
	f.describeCalls++
	return page, nil
}
func (f *fakeEC2Client) StopInstances(ctx context.Context, params *awsec2sdk.StopInstancesInput, optFns ...func(*awsec2sdk.Options)) (*awsec2sdk.StopInstancesOutput, error) {
	f.stopIDs = append([]string(nil), params.InstanceIds...)
	return &awsec2sdk.StopInstancesOutput{StoppingInstances: []ec2types.InstanceStateChange{{InstanceId: aws.String(params.InstanceIds[0]), CurrentState: &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopping}}}}, nil
}
func (f *fakeEC2Client) TerminateInstances(ctx context.Context, params *awsec2sdk.TerminateInstancesInput, optFns ...func(*awsec2sdk.Options)) (*awsec2sdk.TerminateInstancesOutput, error) {
	f.terminateIDs = append([]string(nil), params.InstanceIds...)
	return &awsec2sdk.TerminateInstancesOutput{TerminatingInstances: []ec2types.InstanceStateChange{{InstanceId: aws.String(params.InstanceIds[0]), CurrentState: &ec2types.InstanceState{Name: ec2types.InstanceStateNameShuttingDown}}}}, nil
}

func TestListInstancesMapsEC2Fields(t *testing.T) {
	launch := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	deleteOnTermination := true
	client := &fakeEC2Client{describePages: []*awsec2sdk.DescribeInstancesOutput{
		{Reservations: []ec2types.Reservation{
			{Instances: []ec2types.Instance{
				{
					InstanceId:       aws.String("i-123"),
					InstanceType:     ec2types.InstanceTypeT3Micro,
					State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
					ImageId:          aws.String("ami-123"),
					KeyName:          aws.String("key"),
					LaunchTime:       aws.Time(launch),
					Placement:        &ec2types.Placement{AvailabilityZone: aws.String("eu-west-1a")},
					VpcId:            aws.String("vpc-1"),
					SubnetId:         aws.String("subnet-1"),
					PrivateIpAddress: aws.String("10.0.0.1"),
					PublicIpAddress:  aws.String("1.2.3.4"),
					Tags:             []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("api")}, {Key: aws.String("Env"), Value: aws.String("prod")}},
					SecurityGroups:   []ec2types.GroupIdentifier{{GroupId: aws.String("sg-1"), GroupName: aws.String("web")}},
					BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
						{DeviceName: aws.String("/dev/xvda"), Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-1"), DeleteOnTermination: aws.Bool(deleteOnTermination)}},
					},
				},
			}},
		}},
	}}
	instances, err := newServiceWithClient(client).ListInstances(context.Background(), "dev", "eu-west-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 {
		t.Fatalf("instances=%#v", instances)
	}
	instance := instances[0]
	if instance.ID != "i-123" || instance.Name != "api" || instance.State != "running" || instance.Type != "t3.micro" || instance.AvailabilityZone != "eu-west-1a" || instance.PrivateIP != "10.0.0.1" || instance.PublicIP != "1.2.3.4" {
		t.Fatalf("unexpected instance mapping: %#v", instance)
	}
	if len(instance.SecurityGroups) != 1 || instance.SecurityGroups[0].Name != "web" || len(instance.BlockDevices) != 1 || !instance.BlockDevices[0].DeleteOnTermination {
		t.Fatalf("nested mapping failed: %#v", instance)
	}
}

func TestStopAndTerminateInstances(t *testing.T) {
	client := &fakeEC2Client{}
	svc := newServiceWithClient(client)
	stopped, err := svc.StopInstance(context.Background(), domainec2.StopInstanceInput{ProfileName: "dev", Region: "eu-west-1", InstanceID: "i-123"})
	if err != nil {
		t.Fatal(err)
	}
	terminated, err := svc.TerminateInstance(context.Background(), domainec2.TerminateInstanceInput{ProfileName: "dev", Region: "eu-west-1", InstanceID: "i-123"})
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Instance.State != "stopping" || terminated.Instance.State != "shutting-down" || len(client.stopIDs) != 1 || len(client.terminateIDs) != 1 {
		t.Fatalf("unexpected actions: stopped=%#v terminated=%#v client=%#v", stopped, terminated, client)
	}
}
