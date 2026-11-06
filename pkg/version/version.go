package version

// Framework version. Bumped by a release-prep commit before every release
// (rc, minor, major, patch) — see docs/operations/release-policy.md.

const (
	// Version is the full semantic version of the framework, without the
	// "v" prefix and including any pre-release suffix (e.g. "0.2.0-rc.1").
	Version = "0.2.0-rc.1"

	// Major is the major version component of the framework.
	Major = 0

	// Minor is the minor version component of the framework.
	Minor = 2
)

// GetVersion returns the current version of the framework
func GetVersion() string {
	return Version
}
