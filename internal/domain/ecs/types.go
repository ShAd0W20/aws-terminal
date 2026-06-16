package ecs

import "time"

type Cluster struct {
	Name                    string
	ARN                     string
	Status                  string
	ActiveServicesCount     int
	RunningTasksCount       int
	PendingTasksCount       int
	RegisteredInstanceCount int
}

type TaskDefinitionSummary struct {
	ARN         string
	DisplayName string
	Family      string
	Revision    int
	Status      string
}

type UpdateServiceInput struct {
	ProfileName        string
	Region             string
	ClusterARN         string
	Service            string
	TaskDefinitionARN  string
	DesiredCount       *int
	ForceNewDeployment bool
}

type UpdateServiceResult struct {
	Service Service
}

type Service struct {
	Name                 string
	ARN                  string
	Status               string
	TaskDefinitionARN    string
	TaskDefinition       string
	DesiredCount         int
	RunningCount         int
	PendingCount         int
	LaunchType           string
	CapacityProviders    []string
	PlatformVersion      string
	CreatedAt            time.Time
	DeploymentController string
	Deployments          []Deployment
	SubnetCount          int
	SecurityGroupCount   int
	AssignPublicIP       string
}

type Deployment struct {
	Status            string
	RolloutState      string
	TaskDefinitionARN string
	TaskDefinition    string
	DesiredCount      int
	RunningCount      int
	PendingCount      int
}

type Task struct {
	ID                string
	ARN               string
	LastStatus        string
	DesiredStatus     string
	HealthStatus      string
	TaskDefinitionARN string
	TaskDefinition    string
	Group             string
	LaunchType        string
	PlatformVersion   string
	AvailabilityZone  string
	Connectivity      string
	PrivateIP         string
	CreatedAt         time.Time
	PullStartedAt     time.Time
	PullStoppedAt     time.Time
	StartedAt         time.Time
	StoppingAt        time.Time
	StoppedAt         time.Time
	StoppedReason     string
	Containers        []Container
	Attachments       []Attachment
}

type Container struct {
	Name       string
	Image      string
	LastStatus string
	ExitCode   *int
	Reason     string
}

type Attachment struct {
	ENI       string
	Subnet    string
	MAC       string
	PrivateIP string
}

type LogTarget struct {
	ContainerName string
	LogGroup      string
	LogStream     string
	StreamPrefix  string
	TaskID        string
	Region        string
	Supported     bool
	Message       string
}

type LogEvent struct {
	ID        string
	Timestamp time.Time
	Message   string
}

type LogEventsPage struct {
	Events           []LogEvent
	NextForwardToken string
	LogStream        string
}
