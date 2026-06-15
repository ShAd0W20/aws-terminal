package update

import (
	"context"

	domainupdate "aws-terminal/internal/domain/update"
)

type ReleaseSource interface {
	LatestRelease(ctx context.Context) (domainupdate.Release, error)
}

type Installer interface {
	Install(ctx context.Context, release domainupdate.Release, currentVersion string) (domainupdate.InstallResult, error)
	InstallInstructions(ctx context.Context) (domainupdate.InstallResult, error)
}
