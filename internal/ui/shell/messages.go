package shell

import (
	domainauth "aws-terminal/internal/domain/auth"
	domainprofile "aws-terminal/internal/domain/profile"
	domainsession "aws-terminal/internal/domain/session"
	domainupdate "aws-terminal/internal/domain/update"
)

type profilesLoadedMsg struct {
	profiles []domainprofile.Profile
	err      error
}

type ssoSessionCheckedMsg struct {
	profile  domainprofile.Profile
	reusable bool
	err      error
}

type ssoLoginStartedMsg struct {
	prompt domainauth.Prompt
	err    error
}

type ssoLoginPolledMsg struct {
	sessionID string
	result    domainauth.PollResult
	err       error
}

type profileActivatedMsg struct {
	session domainsession.Session
	err     error
}

type updateCheckedMsg struct {
	result  domainupdate.CheckResult
	startup bool
	err     error
}

type updateInstalledMsg struct {
	result domainupdate.InstallResult
	err    error
}
