package ec2

import "time"

type Instance struct {
	ID                 string
	Name               string
	State              string
	Type               string
	Architecture       string
	Platform           string
	ImageID            string
	KeyName            string
	LaunchTime         time.Time
	AvailabilityZone   string
	VpcID              string
	SubnetID           string
	PrivateIP          string
	PublicIP           string
	PrivateDNS         string
	PublicDNS          string
	IAMInstanceProfile string
	SecurityGroups     []SecurityGroup
	NetworkInterfaces  []NetworkInterface
	BlockDevices       []BlockDevice
	Tags               []Tag
}

type SecurityGroup struct {
	ID   string
	Name string
}

type NetworkInterface struct {
	ID          string
	SubnetID    string
	VpcID       string
	PrivateIP   string
	PublicIP    string
	Description string
	Status      string
}

type BlockDevice struct {
	DeviceName          string
	VolumeID            string
	DeleteOnTermination bool
}

type Tag struct {
	Key   string
	Value string
}

type StopInstanceInput struct {
	ProfileName string
	Region      string
	InstanceID  string
}

type StopInstanceResult struct {
	Instance Instance
}

type TerminateInstanceInput struct {
	ProfileName string
	Region      string
	InstanceID  string
}

type TerminateInstanceResult struct {
	Instance Instance
}
