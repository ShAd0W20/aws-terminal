package awsecs

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awslogssdk "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	awsec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	awsecsdk "github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	appsecs "aws-terminal/internal/application/ecs"
	domainecs "aws-terminal/internal/domain/ecs"
	"aws-terminal/internal/infrastructure/awsclients"
)

type Service struct{ clients *awsclients.Factory }

func NewService() *Service { return NewServiceWithFactory(awsclients.Default()) }
func NewServiceWithFactory(clients *awsclients.Factory) *Service {
	if clients == nil {
		clients = awsclients.Default()
	}
	return &Service{clients: clients}
}

func (s *Service) ListClusters(ctx context.Context, profileName, region string) ([]domainecs.Cluster, error) {
	ctx, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
	defer cancel()
	client, err := s.client(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	arns := []string{}
	p := awsecsdk.NewListClustersPaginator(client, &awsecsdk.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, page.ClusterArns...)
	}
	clusters := []domainecs.Cluster{}
	for _, batch := range chunks(arns, 100) {
		out, err := client.DescribeClusters(ctx, &awsecsdk.DescribeClustersInput{Clusters: batch})
		if err != nil {
			return nil, err
		}
		for _, c := range out.Clusters {
			clusters = append(clusters, clusterFromSDK(c))
		}
	}
	return clusters, nil
}

func (s *Service) ListServices(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Service, error) {
	ctx, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
	defer cancel()
	client, err := s.client(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	arns := []string{}
	p := awsecsdk.NewListServicesPaginator(client, &awsecsdk.ListServicesInput{Cluster: aws.String(strings.TrimSpace(clusterARN))})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, page.ServiceArns...)
	}
	services := []domainecs.Service{}
	for _, batch := range chunks(arns, 10) {
		out, err := client.DescribeServices(ctx, &awsecsdk.DescribeServicesInput{Cluster: aws.String(strings.TrimSpace(clusterARN)), Services: batch})
		if err != nil {
			return nil, err
		}
		for _, svc := range out.Services {
			services = append(services, serviceFromSDK(svc))
		}
	}
	return services, nil
}

func (s *Service) ListTasks(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Task, error) {
	ctx, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
	defer cancel()
	client, err := s.client(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	arns := []string{}
	p := awsecsdk.NewListTasksPaginator(client, &awsecsdk.ListTasksInput{Cluster: aws.String(strings.TrimSpace(clusterARN))})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, page.TaskArns...)
	}
	tasks := []domainecs.Task{}
	containerInstanceByTaskARN := map[string]string{}
	for _, batch := range chunks(arns, 100) {
		out, err := client.DescribeTasks(ctx, &awsecsdk.DescribeTasksInput{Cluster: aws.String(strings.TrimSpace(clusterARN)), Tasks: batch})
		if err != nil {
			return nil, err
		}
		for _, task := range out.Tasks {
			domainTask := taskFromSDK(task)
			if domainTask.PrivateIP == "" && task.ContainerInstanceArn != nil {
				containerInstanceByTaskARN[domainTask.ARN] = aws.ToString(task.ContainerInstanceArn)
			}
			tasks = append(tasks, domainTask)
		}
	}
	if err := s.fillContainerInstancePrivateIPs(ctx, profileName, region, strings.TrimSpace(clusterARN), tasks, containerInstanceByTaskARN); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Service) DescribeTaskLogTargets(ctx context.Context, profileName, region, taskDefinitionARN, taskID string) ([]domainecs.LogTarget, error) {
	ctx, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
	defer cancel()
	client, err := s.client(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	out, err := client.DescribeTaskDefinition(ctx, &awsecsdk.DescribeTaskDefinitionInput{TaskDefinition: aws.String(strings.TrimSpace(taskDefinitionARN))})
	if err != nil {
		return nil, err
	}
	if out.TaskDefinition == nil {
		return nil, fmt.Errorf("task definition not found")
	}
	return logTargetsFromTaskDefinition(out.TaskDefinition.ContainerDefinitions, strings.TrimSpace(taskID)), nil
}

func (s *Service) FetchTaskLogEvents(ctx context.Context, profileName, region string, target domainecs.LogTarget, nextToken string, lookback time.Duration, limit int32) (domainecs.LogEventsPage, error) {
	logsRegion := strings.TrimSpace(target.Region)
	if logsRegion == "" {
		logsRegion = region
	}
	ctx, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
	defer cancel()
	client, err := s.clients.CloudWatchLogs(ctx, profileName, logsRegion)
	if err != nil {
		return domainecs.LogEventsPage{}, fmt.Errorf("load CloudWatch Logs client: %w", err)
	}
	input := &awslogssdk.GetLogEventsInput{
		LogGroupName:  aws.String(strings.TrimSpace(target.LogGroup)),
		LogStreamName: aws.String(strings.TrimSpace(target.LogStream)),
		Limit:         aws.Int32(limit),
		StartFromHead: aws.Bool(true),
	}
	if strings.TrimSpace(nextToken) != "" {
		input.NextToken = aws.String(strings.TrimSpace(nextToken))
	} else if lookback > 0 {
		input.StartTime = aws.Int64(time.Now().Add(-lookback).UnixMilli())
	}
	page, err := getLogEvents(ctx, client, input, strings.TrimSpace(target.LogStream))
	if err == nil {
		return page, nil
	}
	if !isResourceNotFound(err) {
		return domainecs.LogEventsPage{}, err
	}
	resolvedStream, resolveErr := s.findTaskLogStream(ctx, client, target)
	if resolveErr != nil || resolvedStream == "" || resolvedStream == strings.TrimSpace(target.LogStream) {
		return domainecs.LogEventsPage{}, err
	}
	input.LogStreamName = aws.String(resolvedStream)
	input.NextToken = nil
	if lookback > 0 {
		input.StartTime = aws.Int64(time.Now().Add(-lookback).UnixMilli())
	}
	return getLogEvents(ctx, client, input, resolvedStream)
}

func getLogEvents(ctx context.Context, client *awslogssdk.Client, input *awslogssdk.GetLogEventsInput, logStream string) (domainecs.LogEventsPage, error) {
	out, err := client.GetLogEvents(ctx, input)
	if err != nil {
		return domainecs.LogEventsPage{}, err
	}
	page := domainecs.LogEventsPage{NextForwardToken: aws.ToString(out.NextForwardToken), LogStream: logStream}
	for _, event := range out.Events {
		message := aws.ToString(event.Message)
		timestamp := aws.ToInt64(event.Timestamp)
		id := fmt.Sprintf("%d/%d/%s", timestamp, aws.ToInt64(event.IngestionTime), message)
		page.Events = append(page.Events, domainecs.LogEvent{ID: id, Timestamp: time.UnixMilli(timestamp), Message: message})
	}
	return page, nil
}

func isResourceNotFound(err error) bool {
	var apiErr smithy.APIError
	return err != nil && errors.As(err, &apiErr) && apiErr.ErrorCode() == "ResourceNotFoundException"
}

func (s *Service) findTaskLogStream(ctx context.Context, client *awslogssdk.Client, target domainecs.LogTarget) (string, error) {
	for _, prefix := range logStreamSearchPrefixes(target) {
		streams := []string{}
		p := awslogssdk.NewDescribeLogStreamsPaginator(client, &awslogssdk.DescribeLogStreamsInput{LogGroupName: aws.String(strings.TrimSpace(target.LogGroup)), LogStreamNamePrefix: aws.String(prefix)})
		for p.HasMorePages() {
			page, err := p.NextPage(ctx)
			if err != nil {
				return "", err
			}
			for _, stream := range page.LogStreams {
				streams = append(streams, aws.ToString(stream.LogStreamName))
			}
			if match := matchingTaskLogStream(streams, target); match != "" {
				return match, nil
			}
		}
	}
	return "", nil
}

var _ appsecs.API = (*Service)(nil)

func (s *Service) client(ctx context.Context, profileName, region string) (*awsecsdk.Client, error) {
	client, err := s.clients.ECS(ctx, profileName, region)
	if err != nil {
		return nil, fmt.Errorf("load ECS client: %w", err)
	}
	return client, nil
}

func (s *Service) fillContainerInstancePrivateIPs(ctx context.Context, profileName, region, clusterARN string, tasks []domainecs.Task, containerInstanceByTaskARN map[string]string) error {
	if len(containerInstanceByTaskARN) == 0 {
		return nil
	}
	ecsClient, err := s.clients.ECS(ctx, profileName, region)
	if err != nil {
		return fmt.Errorf("load ECS client: %w", err)
	}
	ec2Client, err := s.clients.EC2(ctx, profileName, region)
	if err != nil {
		return fmt.Errorf("load EC2 client: %w", err)
	}
	containerARNs := uniqueValues(containerInstanceByTaskARN)
	containerToEC2 := map[string]string{}
	for _, batch := range chunks(containerARNs, 100) {
		out, err := ecsClient.DescribeContainerInstances(ctx, &awsecsdk.DescribeContainerInstancesInput{Cluster: aws.String(clusterARN), ContainerInstances: batch})
		if err != nil {
			return err
		}
		for _, instance := range out.ContainerInstances {
			containerToEC2[aws.ToString(instance.ContainerInstanceArn)] = aws.ToString(instance.Ec2InstanceId)
		}
	}
	ec2IDs := uniqueValues(containerToEC2)
	ec2PrivateIPs, err := describeEC2PrivateIPs(ctx, ec2Client, ec2IDs)
	if err != nil {
		return err
	}
	for i := range tasks {
		if tasks[i].PrivateIP != "" {
			continue
		}
		containerARN := containerInstanceByTaskARN[tasks[i].ARN]
		ec2ID := containerToEC2[containerARN]
		if ip := ec2PrivateIPs[ec2ID]; ip != "" {
			tasks[i].PrivateIP = ip
		}
	}
	return nil
}

func describeEC2PrivateIPs(ctx context.Context, client *awsec2sdk.Client, instanceIDs []string) (map[string]string, error) {
	privateIPs := map[string]string{}
	for _, batch := range chunks(instanceIDs, 1000) {
		out, err := client.DescribeInstances(ctx, &awsec2sdk.DescribeInstancesInput{InstanceIds: batch})
		if err != nil {
			return nil, err
		}
		for _, reservation := range out.Reservations {
			for _, instance := range reservation.Instances {
				privateIPs[aws.ToString(instance.InstanceId)] = aws.ToString(instance.PrivateIpAddress)
			}
		}
	}
	return privateIPs, nil
}

func clusterFromSDK(c ecstypes.Cluster) domainecs.Cluster {
	return domainecs.Cluster{Name: aws.ToString(c.ClusterName), ARN: aws.ToString(c.ClusterArn), Status: aws.ToString(c.Status), ActiveServicesCount: int(c.ActiveServicesCount), RunningTasksCount: int(c.RunningTasksCount), PendingTasksCount: int(c.PendingTasksCount), RegisteredInstanceCount: int(c.RegisteredContainerInstancesCount)}
}

func serviceFromSDK(s ecstypes.Service) domainecs.Service {
	deployments := make([]domainecs.Deployment, 0, len(s.Deployments))
	for _, d := range s.Deployments {
		deployments = append(deployments, domainecs.Deployment{Status: aws.ToString(d.Status), RolloutState: string(d.RolloutState), TaskDefinitionARN: aws.ToString(d.TaskDefinition), TaskDefinition: taskDefinitionName(aws.ToString(d.TaskDefinition)), DesiredCount: int(d.DesiredCount), RunningCount: int(d.RunningCount), PendingCount: int(d.PendingCount)})
	}
	providers := make([]string, 0, len(s.CapacityProviderStrategy))
	for _, cp := range s.CapacityProviderStrategy {
		providers = append(providers, aws.ToString(cp.CapacityProvider))
	}
	subnets, sgs, publicIP := 0, 0, ""
	if s.NetworkConfiguration != nil && s.NetworkConfiguration.AwsvpcConfiguration != nil {
		subnets = len(s.NetworkConfiguration.AwsvpcConfiguration.Subnets)
		sgs = len(s.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups)
		publicIP = string(s.NetworkConfiguration.AwsvpcConfiguration.AssignPublicIp)
	}
	controller := ""
	if s.DeploymentController != nil {
		controller = string(s.DeploymentController.Type)
	}
	return domainecs.Service{Name: aws.ToString(s.ServiceName), ARN: aws.ToString(s.ServiceArn), Status: aws.ToString(s.Status), TaskDefinitionARN: aws.ToString(s.TaskDefinition), TaskDefinition: taskDefinitionName(aws.ToString(s.TaskDefinition)), DesiredCount: int(s.DesiredCount), RunningCount: int(s.RunningCount), PendingCount: int(s.PendingCount), LaunchType: string(s.LaunchType), CapacityProviders: providers, PlatformVersion: aws.ToString(s.PlatformVersion), CreatedAt: aws.ToTime(s.CreatedAt), DeploymentController: controller, Deployments: deployments, SubnetCount: subnets, SecurityGroupCount: sgs, AssignPublicIP: publicIP}
}

func taskFromSDK(t ecstypes.Task) domainecs.Task {
	containers := make([]domainecs.Container, 0, len(t.Containers))
	privateIP := ""
	for _, c := range t.Containers {
		var exit *int
		if c.ExitCode != nil {
			v := int(aws.ToInt32(c.ExitCode))
			exit = &v
		}
		if privateIP == "" {
			privateIP = containerPrivateIP(c)
		}
		containers = append(containers, domainecs.Container{Name: aws.ToString(c.Name), Image: aws.ToString(c.Image), LastStatus: aws.ToString(c.LastStatus), ExitCode: exit, Reason: aws.ToString(c.Reason)})
	}
	attachments := make([]domainecs.Attachment, 0, len(t.Attachments))
	for _, a := range t.Attachments {
		att := attachmentFromSDK(a)
		if privateIP == "" {
			privateIP = att.PrivateIP
		}
		attachments = append(attachments, att)
	}
	arn := aws.ToString(t.TaskArn)
	return domainecs.Task{ID: taskID(arn), ARN: arn, LastStatus: aws.ToString(t.LastStatus), DesiredStatus: aws.ToString(t.DesiredStatus), HealthStatus: string(t.HealthStatus), TaskDefinitionARN: aws.ToString(t.TaskDefinitionArn), TaskDefinition: taskDefinitionName(aws.ToString(t.TaskDefinitionArn)), Group: aws.ToString(t.Group), LaunchType: string(t.LaunchType), PlatformVersion: aws.ToString(t.PlatformVersion), AvailabilityZone: aws.ToString(t.AvailabilityZone), Connectivity: string(t.Connectivity), PrivateIP: privateIP, CreatedAt: aws.ToTime(t.CreatedAt), PullStartedAt: aws.ToTime(t.PullStartedAt), PullStoppedAt: aws.ToTime(t.PullStoppedAt), StartedAt: aws.ToTime(t.StartedAt), StoppingAt: aws.ToTime(t.StoppingAt), StoppedAt: aws.ToTime(t.StoppedAt), StoppedReason: aws.ToString(t.StoppedReason), Containers: containers, Attachments: attachments}
}

func containerPrivateIP(c ecstypes.Container) string {
	for _, networkInterface := range c.NetworkInterfaces {
		if ip := strings.TrimSpace(aws.ToString(networkInterface.PrivateIpv4Address)); ip != "" {
			return ip
		}
	}
	return ""
}

func attachmentFromSDK(a ecstypes.Attachment) domainecs.Attachment {
	att := domainecs.Attachment{}
	for _, d := range a.Details {
		switch aws.ToString(d.Name) {
		case "networkInterfaceId":
			att.ENI = aws.ToString(d.Value)
		case "subnetId":
			att.Subnet = aws.ToString(d.Value)
		case "macAddress":
			att.MAC = aws.ToString(d.Value)
		case "privateIPv4Address":
			att.PrivateIP = aws.ToString(d.Value)
		}
	}
	return att
}

func logTargetsFromTaskDefinition(containers []ecstypes.ContainerDefinition, taskID string) []domainecs.LogTarget {
	taskID = strings.Trim(strings.TrimSpace(taskID), "/")
	targets := make([]domainecs.LogTarget, 0, len(containers))
	for _, c := range containers {
		name := aws.ToString(c.Name)
		target := domainecs.LogTarget{ContainerName: name, TaskID: taskID, Supported: false, Message: fmt.Sprintf("No awslogs CloudWatch Logs configuration found for container %s.", valueOrFallback(name, "unknown"))}
		if c.LogConfiguration == nil || c.LogConfiguration.LogDriver != ecstypes.LogDriverAwslogs {
			targets = append(targets, target)
			continue
		}
		options := c.LogConfiguration.Options
		group := strings.TrimSpace(options["awslogs-group"])
		logsRegion := strings.TrimSpace(options["awslogs-region"])
		prefix := strings.TrimSpace(options["awslogs-stream-prefix"])
		switch {
		case group == "":
			target.Message = fmt.Sprintf("Container %s uses awslogs but has no awslogs-group.", valueOrFallback(name, "unknown"))
		case prefix == "":
			target.Message = fmt.Sprintf("Container %s uses awslogs but has no awslogs-stream-prefix.", valueOrFallback(name, "unknown"))
		case taskID == "":
			target.Message = fmt.Sprintf("Container %s uses awslogs but task ID is missing.", valueOrFallback(name, "unknown"))
		default:
			target.LogGroup = group
			target.Region = logsRegion
			target.StreamPrefix = prefix
			target.LogStream = awslogsStreamName(prefix, name, taskID)
			target.Supported = true
			target.Message = ""
		}
		targets = append(targets, target)
	}
	return targets
}

func awslogsStreamName(prefix, containerName, taskID string) string {
	prefix = strings.TrimRight(strings.TrimSpace(prefix), "/")
	containerName = strings.Trim(strings.TrimSpace(containerName), "/")
	taskID = strings.Trim(strings.TrimSpace(taskID), "/")
	return prefix + "/" + containerName + "/" + taskID
}

func logStreamSearchPrefixes(target domainecs.LogTarget) []string {
	seen := map[string]struct{}{}
	add := func(value string, out *[]string) {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		*out = append(*out, value)
	}
	prefixes := []string{}
	add(target.LogStream, &prefixes)
	if target.StreamPrefix != "" && target.ContainerName != "" {
		add(strings.TrimRight(strings.TrimSpace(target.StreamPrefix), "/")+"/"+strings.Trim(strings.TrimSpace(target.ContainerName), "/"), &prefixes)
	}
	add(target.StreamPrefix, &prefixes)
	return prefixes
}

func matchingTaskLogStream(streams []string, target domainecs.LogTarget) string {
	exact := strings.TrimSpace(target.LogStream)
	taskID := strings.Trim(strings.TrimSpace(target.TaskID), "/")
	containerName := strings.Trim(strings.TrimSpace(target.ContainerName), "/")
	for _, stream := range streams {
		if exact != "" && stream == exact {
			return stream
		}
	}
	if taskID == "" {
		return ""
	}
	containerSegment := "/" + containerName + "/"
	for _, stream := range streams {
		if containerName != "" && strings.Contains(stream, containerSegment) && strings.HasSuffix(stream, "/"+taskID) {
			return stream
		}
	}
	for _, stream := range streams {
		if strings.HasSuffix(stream, "/"+taskID) || stream == taskID {
			return stream
		}
	}
	for _, stream := range streams {
		if strings.Contains(stream, taskID) {
			return stream
		}
	}
	return ""
}

func valueOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func taskDefinitionName(arn string) string {
	if arn == "" {
		return ""
	}
	return path.Base(arn)
}
func taskID(arn string) string {
	if arn == "" {
		return ""
	}
	return path.Base(arn)
}

func uniqueValues(valuesByKey map[string]string) []string {
	seen := map[string]struct{}{}
	values := make([]string, 0, len(valuesByKey))
	for _, value := range valuesByKey {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func chunks(values []string, size int) [][]string {
	if len(values) == 0 {
		return nil
	}
	out := make([][]string, 0, (len(values)+size-1)/size)
	for len(values) > 0 {
		n := size
		if len(values) < n {
			n = len(values)
		}
		out = append(out, values[:n])
		values = values[n:]
	}
	return out
}
