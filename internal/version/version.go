package version

var version = "v0.0.0"

// Value returns the build-time injected CLI version.
func Value() string {
	return version
}
