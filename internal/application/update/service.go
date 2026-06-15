package update

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domainupdate "aws-terminal/internal/domain/update"
)

const DevVersion = "dev"

type Service struct {
	currentVersion string
	releases       ReleaseSource
	installer      Installer
}

func NewService(currentVersion string, releases ReleaseSource, installer Installer) *Service {
	currentVersion = strings.TrimSpace(currentVersion)
	if currentVersion == "" {
		currentVersion = DevVersion
	}
	return &Service{currentVersion: currentVersion, releases: releases, installer: installer}
}

func (s *Service) CurrentVersion() string {
	if s == nil || strings.TrimSpace(s.currentVersion) == "" {
		return DevVersion
	}
	return s.currentVersion
}

func (s *Service) Check(ctx context.Context) (domainupdate.CheckResult, error) {
	if s == nil || s.releases == nil {
		return domainupdate.CheckResult{}, fmt.Errorf("update service is not configured")
	}

	release, err := s.releases.LatestRelease(ctx)
	if err != nil {
		return domainupdate.CheckResult{}, err
	}

	current := s.CurrentVersion()
	development := IsDevelopmentVersion(current)
	available := !development && CompareVersions(current, release.Version) < 0
	return domainupdate.CheckResult{
		CurrentVersion:   current,
		LatestVersion:    release.Version,
		ReleaseURL:       release.URL,
		UpdateAvailable:  available,
		DevelopmentBuild: development,
	}, nil
}

func (s *Service) InstallLatest(ctx context.Context) (domainupdate.InstallResult, error) {
	if s == nil || s.releases == nil || s.installer == nil {
		return domainupdate.InstallResult{}, fmt.Errorf("update service is not configured")
	}

	release, err := s.releases.LatestRelease(ctx)
	if err != nil {
		return domainupdate.InstallResult{}, err
	}

	current := s.CurrentVersion()
	if !IsDevelopmentVersion(current) && CompareVersions(current, release.Version) >= 0 {
		result, err := s.installer.InstallInstructions(ctx)
		if err != nil {
			return domainupdate.InstallResult{}, err
		}
		result.CurrentVersion = current
		result.LatestVersion = release.Version
		result.Updated = false
		return result, nil
	}

	return s.installer.Install(ctx, release, current)
}

func IsDevelopmentVersion(version string) bool {
	version = strings.TrimSpace(strings.ToLower(version))
	return version == "" || version == DevVersion || version == "(devel)"
}

func CompareVersions(a, b string) int {
	av := parseVersion(a)
	bv := parseVersion(b)
	limit := len(av.parts)
	if len(bv.parts) > limit {
		limit = len(bv.parts)
	}
	for i := 0; i < limit; i++ {
		ai, bi := 0, 0
		if i < len(av.parts) {
			ai = av.parts[i]
		}
		if i < len(bv.parts) {
			bi = bv.parts[i]
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return strings.Compare(av.suffix, bv.suffix)
}

type parsedVersion struct {
	parts  []int
	suffix string
}

func parseVersion(version string) parsedVersion {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	version = strings.TrimPrefix(version, "V")
	mainPart, suffix, _ := strings.Cut(version, "-")
	pieces := strings.Split(mainPart, ".")
	parts := make([]int, 0, len(pieces))
	for _, piece := range pieces {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			parts = append(parts, 0)
			continue
		}
		value, err := strconv.Atoi(piece)
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, value)
	}
	return parsedVersion{parts: parts, suffix: suffix}
}
