package update

const (
	InstallMethodDirect   = "direct"
	InstallMethodHomebrew = "homebrew"
	InstallMethodScoop    = "scoop"
	InstallMethodWindows  = "windows"
	InstallMethodUnknown  = "unknown"
)

type Asset struct {
	Name string
	URL  string
}

type Release struct {
	Version string
	URL     string
	Assets  []Asset
}

type CheckResult struct {
	CurrentVersion   string
	LatestVersion    string
	ReleaseURL       string
	UpdateAvailable  bool
	DevelopmentBuild bool
}

type InstallResult struct {
	CurrentVersion  string
	LatestVersion   string
	InstallMethod   string
	Updated         bool
	SelfUpdatable   bool
	ExecutablePath  string
	Instructions    string
	RestartRequired bool
}
