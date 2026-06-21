package app

import (
	"aws-terminal/internal/config"
	"aws-terminal/internal/ui/pages"
	cloudfrontpage "aws-terminal/internal/ui/pages/cloudfront"
	ecrpage "aws-terminal/internal/ui/pages/ecr"
	ecspage "aws-terminal/internal/ui/pages/ecs"
	s3page "aws-terminal/internal/ui/pages/s3"
	sqspage "aws-terminal/internal/ui/pages/sqs"
)

func DefaultPages(s3Service s3page.S3Service, cloudFrontService cloudfrontpage.CloudFrontService, ecrService ecrpage.ECRService, ecsService ecspage.ECSService, sqsService sqspage.SQSService, preferenceStore config.PreferenceStore) []pages.Page {
	return []pages.Page{
		pages.NewDashboardPage(),
		s3page.NewS3PageWithPreferences(s3Service, preferenceStore),
		cloudfrontpage.NewCloudFrontPage(cloudFrontService),
		ecrpage.NewECRPage(ecrService),
		ecspage.NewECSPage(ecsService),
		sqspage.NewSQSPage(sqsService),
	}
}
