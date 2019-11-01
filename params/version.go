package params

import (
	"fmt"
)

const (
	VersionMajor = 0 // Major version component of the current release
	VersionMinor = 2 // Minor version component of the current release
	VersionPatch = 1 // Patch version component of the current release
)

// Version holds the textual version string.
var Version = func() string {
	return fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)
}()

func VersionFull(versionMeta string, gitCommit string) string {
	vsn := Version + "-" + versionMeta
	if len(gitCommit) >= 8 {
		vsn += "-" + gitCommit[:8]
	} else if len(gitCommit) >= 4 {
		vsn += "-" + gitCommit
	}
	return vsn
}
