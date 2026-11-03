// Package version is the version of this st binary: the release it was built
// as, the module version 'go install' recorded, or "dev" for a build from a
// checkout.
package version

import (
	"regexp"
	"runtime/debug"
	"strconv"
)

// stamped is set by a release build:
//
//	go build -ldflags "-X github.com/cpave3/staccato/pkg/version.stamped=v1.2.3"
var stamped string

// readBuildInfo is debug.ReadBuildInfo, replaced in tests.
var readBuildInfo = debug.ReadBuildInfo

var stable = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// String is the version this binary was released as. Without a stamped
// version it is the main module's version from the Go build information when
// that is a stable vX.Y.Z (as 'go install …@v1.2.3' records), else "dev".
func String() string {
	if stamped != "" {
		return stamped
	}
	if v, ok := moduleVersion(); ok {
		return v
	}
	return "dev"
}

// IsRelease reports whether this binary is a stable release (a stamped
// vX.Y.Z). Only a release updates itself: a 'go install' build lives where
// 'go install' manages it, and a build from source is left to its developer.
func IsRelease() bool {
	return IsStable(stamped)
}

// FromModule reports whether this binary has no stamped version and reports
// the module version Go recorded, as a 'go install …@vX.Y.Z' build does.
func FromModule() bool {
	if stamped != "" {
		return false
	}
	_, ok := moduleVersion()
	return ok
}

func moduleVersion() (string, bool) {
	info, ok := readBuildInfo()
	if !ok || info == nil || !IsStable(info.Main.Version) {
		return "", false
	}
	return info.Main.Version, true
}

// IsStable reports whether v is a stable version, vX.Y.Z.
func IsStable(v string) bool {
	return stable.MatchString(v)
}

// Newer reports whether candidate is a stable version later than current,
// itself a stable version. Anything else is never newer.
func Newer(candidate, current string) bool {
	a, okA := parse(candidate)
	b, okB := parse(current)
	if !okA || !okB {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	m := stable.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}, false
	}
	var out [3]int
	for i := range out {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
