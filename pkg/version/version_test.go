package version

import (
	"runtime/debug"
	"testing"
)

func TestABuildFromSourceIsDev(t *testing.T) {
	stamp(t, "")
	buildInfo(t, "(devel)")
	if got := String(); got != "dev" {
		t.Errorf("String() = %q, want dev", got)
	}
	if IsRelease() {
		t.Error("a build from source counts as a release")
	}
}

func TestAStampedReleaseReportsItsVersion(t *testing.T) {
	stamp(t, "v1.4.2")
	if got := String(); got != "v1.4.2" {
		t.Errorf("String() = %q, want v1.4.2", got)
	}
	if !IsRelease() {
		t.Error("v1.4.2 does not count as a release")
	}
}

func TestOnlyAStableStampCountsAsARelease(t *testing.T) {
	for _, v := range []string{"1.4.2", "v1.4", "v1.4.2-rc.1", "v1.4.2+dirty", "dev", "vX.Y.Z"} {
		stamp(t, v)
		if IsRelease() {
			t.Errorf("%q counts as a release", v)
		}
	}
}

func TestAGoInstallBuildReportsItsModuleVersion(t *testing.T) {
	stamp(t, "")
	buildInfo(t, "v0.5.0")
	if got := String(); got != "v0.5.0" {
		t.Errorf("String() = %q, want v0.5.0", got)
	}
	if IsRelease() {
		t.Error("a go install build counts as a release")
	}
	if !FromModule() {
		t.Error("a go install build is not reported as a module version")
	}
}

func TestABuildFromACheckoutOffATagIsDev(t *testing.T) {
	stamp(t, "")
	for _, v := range []string{
		"v0.5.1-0.20261103010203-abcdefabcdef",
		"v0.5.1-0.20261103010203-abcdefabcdef+dirty",
		"v0.5.0+dirty",
		"(devel)",
		"",
	} {
		buildInfo(t, v)
		if got := String(); got != "dev" {
			t.Errorf("build info %q: String() = %q, want dev", v, got)
		}
		if FromModule() {
			t.Errorf("build info %q is reported as a module version", v)
		}
	}
}

func TestNoBuildInfoIsDev(t *testing.T) {
	stamp(t, "")
	old := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	t.Cleanup(func() { readBuildInfo = old })
	if got := String(); got != "dev" {
		t.Errorf("String() = %q, want dev", got)
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		candidate, current string
		want               bool
	}{
		{"v1.2.4", "v1.2.3", true},
		{"v1.3.0", "v1.2.9", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.2", "v1.2.3", false},
		{"v1.2.4-rc.1", "v1.2.3", false},
		{"nonsense", "v1.2.3", false},
		{"v1.2.4", "dev", false},
	}
	for _, c := range cases {
		if got := Newer(c.candidate, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.candidate, c.current, got, c.want)
		}
	}
}

func stamp(t *testing.T, v string) {
	t.Helper()
	old := stamped
	stamped = v
	t.Cleanup(func() { stamped = old })
}

func buildInfo(t *testing.T, mainVersion string) {
	t.Helper()
	old := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Path: "github.com/cpave3/staccato", Version: mainVersion}}, true
	}
	t.Cleanup(func() { readBuildInfo = old })
}
