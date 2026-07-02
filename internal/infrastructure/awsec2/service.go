package awsec2

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	appec2 "aws-terminal/internal/application/ec2"
	domainec2 "aws-terminal/internal/domain/ec2"
	"aws-terminal/internal/infrastructure/awsclients"
)

type ec2Client interface {
	DescribeInstances(ctx context.Context, params *awsec2sdk.DescribeInstancesInput, optFns ...func(*awsec2sdk.Options)) (*awsec2sdk.DescribeInstancesOutput, error)
	StopInstances(ctx context.Context, params *awsec2sdk.StopInstancesInput, optFns ...func(*awsec2sdk.Options)) (*awsec2sdk.StopInstancesOutput, error)
	TerminateInstances(ctx context.Context, params *awsec2sdk.TerminateInstancesInput, optFns ...func(*awsec2sdk.Options)) (*awsec2sdk.TerminateInstancesOutput, error)
}

type Service struct {
	clients *awsclients.Factory
	client  ec2Client
}

func NewService() *Service { return NewServiceWithFactory(awsclients.Default()) }

func NewServiceWithFactory(clients *awsclients.Factory) *Service {
	if clients == nil {
		clients = awsclients.Default()
	}
	return &Service{clients: clients}
}

func newServiceWithClient(client ec2Client) *Service { return &Service{client: client} }

func (s *Service) ListInstances(ctx context.Context, profileName, region string) ([]domainec2.Instance, error) {
	if s.client == nil {
		ctxWithTimeout, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
		defer cancel()
		ctx = ctxWithTimeout
	}
	client, err := s.ec2Client(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	instances := []domainec2.Instance{}
	p := awsec2sdk.NewDescribeInstancesPaginator(client, &awsec2sdk.DescribeInstancesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				instances = append(instances, instanceFromSDK(instance))
			}
		}
	}
	return instances, nil
}

func (s *Service) StopInstance(ctx context.Context, input domainec2.StopInstanceInput) (domainec2.StopInstanceResult, error) {
	if s.client == nil {
		ctxWithTimeout, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
		defer cancel()
		ctx = ctxWithTimeout
	}
	client, err := s.ec2Client(ctx, input.ProfileName, input.Region)
	if err != nil {
		return domainec2.StopInstanceResult{}, err
	}
	out, err := client.StopInstances(ctx, &awsec2sdk.StopInstancesInput{InstanceIds: []string{strings.TrimSpace(input.InstanceID)}})
	if err != nil {
		return domainec2.StopInstanceResult{}, err
	}
	return domainec2.StopInstanceResult{Instance: instanceFromStateChange(input.InstanceID, out.StoppingInstances)}, nil
}

func (s *Service) TerminateInstance(ctx context.Context, input domainec2.TerminateInstanceInput) (domainec2.TerminateInstanceResult, error) {
	if s.client == nil {
		ctxWithTimeout, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
		defer cancel()
		ctx = ctxWithTimeout
	}
	client, err := s.ec2Client(ctx, input.ProfileName, input.Region)
	if err != nil {
		return domainec2.TerminateInstanceResult{}, err
	}
	out, err := client.TerminateInstances(ctx, &awsec2sdk.TerminateInstancesInput{InstanceIds: []string{strings.TrimSpace(input.InstanceID)}})
	if err != nil {
		return domainec2.TerminateInstanceResult{}, err
	}
	return domainec2.TerminateInstanceResult{Instance: instanceFromStateChange(input.InstanceID, out.TerminatingInstances)}, nil
}

var _ appec2.API = (*Service)(nil)

func (s *Service) ec2Client(ctx context.Context, profileName, region string) (ec2Client, error) {
	if s.client != nil {
		return s.client, nil
	}
	client, err := s.clients.EC2(ctx, profileName, region)
	if err != nil {
		return nil, fmt.Errorf("load EC2 client: %w", err)
	}
	return client, nil
}

func instanceFromSDK(instance ec2types.Instance) domainec2.Instance {
	securityGroups := make([]domainec2.SecurityGroup, 0, len(instance.SecurityGroups))
	for _, group := range instance.SecurityGroups {
		securityGroups = append(securityGroups, domainec2.SecurityGroup{ID: aws.ToString(group.GroupId), Name: aws.ToString(group.GroupName)})
	}
	networkInterfaces := make([]domainec2.NetworkInterface, 0, len(instance.NetworkInterfaces))
	for _, networkInterface := range instance.NetworkInterfaces {
		publicIP := ""
		if networkInterface.Association != nil {
			publicIP = aws.ToString(networkInterface.Association.PublicIp)
		}
		networkInterfaces = append(networkInterfaces, domainec2.NetworkInterface{ID: aws.ToString(networkInterface.NetworkInterfaceId), SubnetID: aws.ToString(networkInterface.SubnetId), VpcID: aws.ToString(networkInterface.VpcId), PrivateIP: aws.ToString(networkInterface.PrivateIpAddress), PublicIP: publicIP, Description: aws.ToString(networkInterface.Description), Status: string(networkInterface.Status)})
	}
	blockDevices := make([]domainec2.BlockDevice, 0, len(instance.BlockDeviceMappings))
	for _, device := range instance.BlockDeviceMappings {
		block := domainec2.BlockDevice{DeviceName: aws.ToString(device.DeviceName)}
		if device.Ebs != nil {
			block.VolumeID = aws.ToString(device.Ebs.VolumeId)
			block.DeleteOnTermination = aws.ToBool(device.Ebs.DeleteOnTermination)
		}
		blockDevices = append(blockDevices, block)
	}
	tags := make([]domainec2.Tag, 0, len(instance.Tags))
	name := ""
	for _, tag := range instance.Tags {
		key, value := aws.ToString(tag.Key), aws.ToString(tag.Value)
		if key == "Name" {
			name = value
		}
		tags = append(tags, domainec2.Tag{Key: key, Value: value})
	}
	sort.Slice(tags, func(i, j int) bool { return strings.ToLower(tags[i].Key) < strings.ToLower(tags[j].Key) })
	iamProfile := ""
	if instance.IamInstanceProfile != nil {
		iamProfile = aws.ToString(instance.IamInstanceProfile.Arn)
	}
	platform := aws.ToString(instance.PlatformDetails)
	if strings.TrimSpace(platform) == "" && instance.Platform != "" {
		platform = string(instance.Platform)
	}
	state := ""
	if instance.State != nil {
		state = string(instance.State.Name)
	}
	availabilityZone := ""
	if instance.Placement != nil {
		availabilityZone = aws.ToString(instance.Placement.AvailabilityZone)
	}
	return domainec2.Instance{ID: aws.ToString(instance.InstanceId), Name: name, State: state, Type: string(instance.InstanceType), Architecture: string(instance.Architecture), Platform: platform, ImageID: aws.ToString(instance.ImageId), KeyName: aws.ToString(instance.KeyName), LaunchTime: aws.ToTime(instance.LaunchTime), AvailabilityZone: availabilityZone, VpcID: aws.ToString(instance.VpcId), SubnetID: aws.ToString(instance.SubnetId), PrivateIP: aws.ToString(instance.PrivateIpAddress), PublicIP: aws.ToString(instance.PublicIpAddress), PrivateDNS: aws.ToString(instance.PrivateDnsName), PublicDNS: aws.ToString(instance.PublicDnsName), IAMInstanceProfile: iamProfile, SecurityGroups: securityGroups, NetworkInterfaces: networkInterfaces, BlockDevices: blockDevices, Tags: tags}
}

func instanceFromStateChange(instanceID string, changes []ec2types.InstanceStateChange) domainec2.Instance {
	instance := domainec2.Instance{ID: strings.TrimSpace(instanceID)}
	if len(changes) == 0 {
		return instance
	}
	change := changes[0]
	instance.ID = aws.ToString(change.InstanceId)
	if change.CurrentState != nil {
		instance.State = string(change.CurrentState.Name)
	}
	return instance
}
