// Package version exposes the ezyl3 build version. The release build overrides
// Version via -ldflags "-X ezyl3/internal/version.Version=<tag>"; a plain
// `go build` from source reports the development placeholder.
package version

import "strings"

// Version is the build version. It defaults to a development placeholder and is
// overridden at release time through the Go linker.
var Version = "dev"

// String returns the resolved build version, falling back to the development
// placeholder when the linker injected an empty value.
func String() string {
	if v := strings.TrimSpace(Version); v != "" {
		return v
	}
	return "dev"
}
