package app

import (
	tea "github.com/charmbracelet/bubbletea"

	authapp "aws-terminal/internal/application/authentication"
	appcloudfront "aws-terminal/internal/application/cloudfront"
	appecr "aws-terminal/internal/application/ecr"
	appecs "aws-terminal/internal/application/ecs"
	apps3 "aws-terminal/internal/application/s3"
	appsession "aws-terminal/internal/application/session"
	appsqs "aws-terminal/internal/application/sqs"
	"aws-terminal/internal/config"
	"aws-terminal/internal/infrastructure/awscloudfront"
	"aws-terminal/internal/infrastructure/awsconfig"
	"aws-terminal/internal/infrastructure/awsecr"
	"aws-terminal/internal/infrastructure/awsecs"
	"aws-terminal/internal/infrastructure/awss3"
	"aws-terminal/internal/infrastructure/awssqs"
	"aws-terminal/internal/infrastructure/awssso"
	"aws-terminal/internal/infrastructure/localdocker"
	"aws-terminal/internal/ui/shell"
)

func Run(version string) error {
	profileRepository := awsconfig.NewSharedConfigProfileRepository()
	identityResolver := awsconfig.NewSTSIdentityResolver()
	sessionService := appsession.NewService(profileRepository, identityResolver)
	authService := authapp.NewService(awssso.NewOIDCDeviceFlowAuthenticator())
	s3Service := apps3.NewService(awss3.NewStore())
	cloudFrontService := appcloudfront.NewService(awscloudfront.NewService())
	dockerService, _ := localdocker.NewService()
	ecrService := appecr.NewService(awsecr.NewService(), dockerService)
	ecsService := appecs.NewService(awsecs.NewService())
	sqsService := appsqs.NewService(awssqs.NewService())
	preferenceStore, _ := config.NewFilePreferenceStore()
	updateService := NewUpdateService(version)

	pageRegistry := DefaultPages(s3Service, cloudFrontService, ecrService, ecsService, sqsService, preferenceStore)

	program := tea.NewProgram(
		shell.NewModelWithPreferencesAndUpdates(sessionService, authService, pageRegistry, preferenceStore, updateService),
		tea.WithAltScreen(),
	)

	_, err := program.Run()
	return err
}
