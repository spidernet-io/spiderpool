package version

import (
	"fmt"
	"runtime"
)

var (
	coordinatorVersion   string
	coordinatorGitCommit string
	coordinatorGitBranch string
	coordinatorBuildDate string
)

// CoordinatorBuildDateVersion returns the version at which the binary was built.
func CoordinatorBuildDateVersion() string {
	return coordinatorVersion
}

// CoordinatorGitCommit returns the commit hash at which the binary was built.
func CoordinatorGitCommit() string {
	return coordinatorGitCommit
}

// CoordinatorGitBranch returns the branch at which the binary was built.
func CoordinatorGitBranch() string {
	return coordinatorGitBranch
}

// CoordinatorBuildDate returns the time at which the binary was built.
func CoordinatorBuildDate() string {
	return coordinatorBuildDate
}

// GoString returns the compiler, compiler version and architecture of the build.
func GoString() string {
	return fmt.Sprintf("%s / %s / %s", runtime.Compiler, runtime.Version(), runtime.GOARCH)
}
