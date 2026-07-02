package app

import (
	"aws-terminal/internal/config"
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/pages"
	cloudfrontpage "aws-terminal/internal/ui/pages/cloudfront"
	ec2page "aws-terminal/internal/ui/pages/ec2"
	ecrpage "aws-terminal/internal/ui/pages/ecr"
	ecspage "aws-terminal/internal/ui/pages/ecs"
	s3page "aws-terminal/internal/ui/pages/s3"
	sqspage "aws-terminal/internal/ui/pages/sqs"
)

func DefaultPages(s3Service s3page.S3Service, cloudFrontService cloudfrontpage.CloudFrontService, ecrService ecrpage.ECRService, ecsService ecspage.ECSService, ec2Service ec2page.EC2Service, sqsService sqspage.SQSService, preferenceStore config.PreferenceStore) []pageapi.Page {
	return []pageapi.Page{
		pages.NewDashboardPage(),
		s3page.NewS3PageWithPreferences(s3Service, preferenceStore),
		cloudfrontpage.NewCloudFrontPage(cloudFrontService),
		ecrpage.NewECRPage(ecrService),
		ecspage.NewECSPage(ecsService),
		ec2page.NewEC2Page(ec2Service),
		sqspage.NewSQSPage(sqsService),
	}
}
