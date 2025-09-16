package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version (injected at build time)
	Version = "dev"
	// GitCommit is the git commit hash (injected at build time)
	GitCommit = "unknown"
	// BuildDate is the build date (injected at build time)
	BuildDate = "unknown"
	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
)

// BuildInfo contains all build information
type BuildInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Arch      string `json:"arch"`
}

// GetBuildInfo returns structured build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		Platform:  runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// GetVersion returns a formatted version string
func GetVersion() string {
	if Version == "dev" {
		return fmt.Sprintf("%s (%s)", Version, GitCommit)
	}
	return Version
}

// GetFullVersion returns a detailed version string
func GetFullVersion() string {
	info := GetBuildInfo()
	return fmt.Sprintf("provisioner %s\nCommit: %s\nBuild Date: %s\nGo Version: %s\nPlatform: %s/%s",
		info.Version, info.GitCommit, info.BuildDate, info.GoVersion, info.Platform, info.Arch)
}
