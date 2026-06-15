package awsclients

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awssdkconfig "github.com/aws/aws-sdk-go-v2/config"
	awscloudfrontsdk "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	awscloudwatchlogssdk "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	awsec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	awsecrsdk "github.com/aws/aws-sdk-go-v2/service/ecr"
	awsecssdk "github.com/aws/aws-sdk-go-v2/service/ecs"
	awss3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	DefaultRegion           = "us-east-1"
	DefaultAppID            = "aws-terminal"
	DefaultOperationTimeout = 2 * time.Minute
	DefaultUploadTimeout    = 30 * time.Minute
	DefaultRetryMaxAttempts = 3
)

type Options struct {
	OperationTimeout  time.Duration
	UploadTimeout     time.Duration
	HTTPClientTimeout time.Duration
	RetryMaxAttempts  int
	RetryMode         aws.RetryMode
	AppID             string
}

func DefaultOptions() Options {
	return Options{
		OperationTimeout: DefaultOperationTimeout,
		UploadTimeout:    DefaultUploadTimeout,
		RetryMaxAttempts: DefaultRetryMaxAttempts,
		RetryMode:        aws.RetryModeStandard,
		AppID:            DefaultAppID,
	}
}

func normalizeOptions(options Options) Options {
	defaults := DefaultOptions()
	if options.OperationTimeout == 0 {
		options.OperationTimeout = defaults.OperationTimeout
	}
	if options.UploadTimeout == 0 {
		options.UploadTimeout = defaults.UploadTimeout
	}
	if options.RetryMaxAttempts == 0 {
		options.RetryMaxAttempts = defaults.RetryMaxAttempts
	}
	if options.RetryMode == "" {
		options.RetryMode = defaults.RetryMode
	}
	if strings.TrimSpace(options.AppID) == "" {
		options.AppID = defaults.AppID
	} else {
		options.AppID = strings.TrimSpace(options.AppID)
	}
	return options
}

type Factory struct {
	opts                  Options
	mu                    sync.RWMutex
	configs               map[string]aws.Config
	s3Clients             map[string]*awss3sdk.Client
	cloudFrontClients     map[string]*awscloudfrontsdk.Client
	cloudWatchLogsClients map[string]*awscloudwatchlogssdk.Client
	ecrClients            map[string]*awsecrsdk.Client
	ecsClients            map[string]*awsecssdk.Client
	ec2Clients            map[string]*awsec2sdk.Client
	stsClients            map[string]*sts.Client
}

func NewFactory() *Factory {
	return NewFactoryWithOptions(DefaultOptions())
}

func NewFactoryWithOptions(options Options) *Factory {
	return &Factory{
		opts:                  normalizeOptions(options),
		configs:               map[string]aws.Config{},
		s3Clients:             map[string]*awss3sdk.Client{},
		cloudFrontClients:     map[string]*awscloudfrontsdk.Client{},
		cloudWatchLogsClients: map[string]*awscloudwatchlogssdk.Client{},
		ecrClients:            map[string]*awsecrsdk.Client{},
		ecsClients:            map[string]*awsecssdk.Client{},
		ec2Clients:            map[string]*awsec2sdk.Client{},
		stsClients:            map[string]*sts.Client{},
	}
}

var defaultFactory = NewFactory()

func Default() *Factory {
	return defaultFactory
}

func (f *Factory) Options() Options {
	if f == nil {
		f = Default()
	}
	return f.opts
}

func (f *Factory) OperationTimeout() time.Duration {
	return f.Options().OperationTimeout
}

func (f *Factory) UploadTimeout() time.Duration {
	return f.Options().UploadTimeout
}

func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func NormalizeRegion(region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		return DefaultRegion
	}
	return region
}

func CacheKey(profileName, region string) string {
	return strings.TrimSpace(profileName) + "|" + NormalizeRegion(region)
}

func (f *Factory) Config(ctx context.Context, profileName, region string) (aws.Config, error) {
	if f == nil {
		f = Default()
	}

	profileName = strings.TrimSpace(profileName)
	region = NormalizeRegion(region)
	key := CacheKey(profileName, region)

	f.mu.RLock()
	cfg, ok := f.configs[key]
	f.mu.RUnlock()
	if ok {
		return cfg, nil
	}

	options := []func(*awssdkconfig.LoadOptions) error{
		awssdkconfig.WithRegion(region),
		awssdkconfig.WithRetryMaxAttempts(f.opts.RetryMaxAttempts),
		awssdkconfig.WithRetryMode(f.opts.RetryMode),
		awssdkconfig.WithAppID(f.opts.AppID),
	}
	if f.opts.HTTPClientTimeout > 0 {
		options = append(options, awssdkconfig.WithHTTPClient(awshttp.NewBuildableClient().WithTimeout(f.opts.HTTPClientTimeout)))
	}
	if profileName != "" {
		options = append(options, awssdkconfig.WithSharedConfigProfile(profileName))
	}

	cfg, err := awssdkconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}

	f.mu.Lock()
	if existing, ok := f.configs[key]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.configs[key] = cfg
	f.mu.Unlock()
	return cfg, nil
}

func (f *Factory) S3(ctx context.Context, profileName, region string) (*awss3sdk.Client, error) {
	if f == nil {
		f = Default()
	}
	key := CacheKey(profileName, region)
	f.mu.RLock()
	client, ok := f.s3Clients[key]
	f.mu.RUnlock()
	if ok {
		return client, nil
	}

	cfg, err := f.Config(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	client = awss3sdk.NewFromConfig(cfg)

	f.mu.Lock()
	if existing, ok := f.s3Clients[key]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.s3Clients[key] = client
	f.mu.Unlock()
	return client, nil
}

func (f *Factory) CloudFront(ctx context.Context, profileName, region string) (*awscloudfrontsdk.Client, error) {
	if f == nil {
		f = Default()
	}
	key := CacheKey(profileName, region)
	f.mu.RLock()
	client, ok := f.cloudFrontClients[key]
	f.mu.RUnlock()
	if ok {
		return client, nil
	}

	cfg, err := f.Config(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	client = awscloudfrontsdk.NewFromConfig(cfg)

	f.mu.Lock()
	if existing, ok := f.cloudFrontClients[key]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.cloudFrontClients[key] = client
	f.mu.Unlock()
	return client, nil
}

func (f *Factory) CloudWatchLogs(ctx context.Context, profileName, region string) (*awscloudwatchlogssdk.Client, error) {
	if f == nil {
		f = Default()
	}
	key := CacheKey(profileName, region)
	f.mu.RLock()
	client, ok := f.cloudWatchLogsClients[key]
	f.mu.RUnlock()
	if ok {
		return client, nil
	}

	cfg, err := f.Config(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	client = awscloudwatchlogssdk.NewFromConfig(cfg)

	f.mu.Lock()
	if existing, ok := f.cloudWatchLogsClients[key]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.cloudWatchLogsClients[key] = client
	f.mu.Unlock()
	return client, nil
}

func (f *Factory) ECR(ctx context.Context, profileName, region string) (*awsecrsdk.Client, error) {
	if f == nil {
		f = Default()
	}
	key := CacheKey(profileName, region)
	f.mu.RLock()
	client, ok := f.ecrClients[key]
	f.mu.RUnlock()
	if ok {
		return client, nil
	}

	cfg, err := f.Config(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	client = awsecrsdk.NewFromConfig(cfg)

	f.mu.Lock()
	if existing, ok := f.ecrClients[key]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.ecrClients[key] = client
	f.mu.Unlock()
	return client, nil
}

func (f *Factory) EC2(ctx context.Context, profileName, region string) (*awsec2sdk.Client, error) {
	if f == nil {
		f = Default()
	}
	key := CacheKey(profileName, region)
	f.mu.RLock()
	client, ok := f.ec2Clients[key]
	f.mu.RUnlock()
	if ok {
		return client, nil
	}

	cfg, err := f.Config(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	client = awsec2sdk.NewFromConfig(cfg)

	f.mu.Lock()
	if existing, ok := f.ec2Clients[key]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.ec2Clients[key] = client
	f.mu.Unlock()
	return client, nil
}

func (f *Factory) ECS(ctx context.Context, profileName, region string) (*awsecssdk.Client, error) {
	if f == nil {
		f = Default()
	}
	key := CacheKey(profileName, region)
	f.mu.RLock()
	client, ok := f.ecsClients[key]
	f.mu.RUnlock()
	if ok {
		return client, nil
	}

	cfg, err := f.Config(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	client = awsecssdk.NewFromConfig(cfg)

	f.mu.Lock()
	if existing, ok := f.ecsClients[key]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.ecsClients[key] = client
	f.mu.Unlock()
	return client, nil
}

func (f *Factory) STS(ctx context.Context, profileName, region string) (*sts.Client, aws.Config, error) {
	if f == nil {
		f = Default()
	}
	key := CacheKey(profileName, region)
	cfg, err := f.Config(ctx, profileName, region)
	if err != nil {
		return nil, aws.Config{}, err
	}

	f.mu.RLock()
	client, ok := f.stsClients[key]
	f.mu.RUnlock()
	if ok {
		return client, cfg, nil
	}

	client = sts.NewFromConfig(cfg)
	f.mu.Lock()
	if existing, ok := f.stsClients[key]; ok {
		f.mu.Unlock()
		return existing, cfg, nil
	}
	f.stsClients[key] = client
	f.mu.Unlock()
	return client, cfg, nil
}
