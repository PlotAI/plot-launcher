// Package version reports the build identity of the Plot Launcher. The values
// are injected at build time with -ldflags "-X".
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the released version, for example "v1.2.3".
	Version = "dev"
	// Commit is the source revision the binary was built from.
	Commit = "none"
	// Date is the build date in RFC 3339 form.
	Date = "unknown"
)

// String returns the full build identity.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s)", Version, Commit, Date, runtime.Version())
}
