package app

import (
	appupdate "aws-terminal/internal/application/update"
	"aws-terminal/internal/infrastructure/githubrelease"
	"aws-terminal/internal/infrastructure/selfupdate"
)

const (
	githubOwner = "ShAd0W20"
	githubRepo  = "aws-terminal"
)

func NewUpdateService(version string) *appupdate.Service {
	return appupdate.NewService(version, githubrelease.NewClient(githubOwner, githubRepo), selfupdate.NewInstaller())
}
