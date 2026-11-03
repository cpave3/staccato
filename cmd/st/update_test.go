package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// releaseServer serves the latest release of cpave3/staccato as GitHub's API
// does, with SHA256SUMS computed from its assets.
type releaseServer struct {
	*httptest.Server
	requests atomic.Int32
}

func newReleaseServer(t *testing.T, tag string, assets map[string][]byte, status int) *releaseServer {
	t.Helper()
	s := &releaseServer{}
	sums := ""
	for name, data := range assets {
		sum := sha256.Sum256(data)
		sums += hex.EncodeToString(sum[:]) + "  " + name + "\n"
	}
	all := map[string][]byte{"SHA256SUMS": []byte(sums)}
	for name, data := range assets {
		all[name] = data
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/cpave3/staccato/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		var list []string
		for name := range all {
			list = append(list, fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, name, s.URL+"/download/"+name))
		}
		fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://github.com/cpave3/staccato/releases/tag/%s","assets":[%s]}`, tag, tag, strings.Join(list, ","))
	})
	mux.HandleFunc("GET /download/{name}", func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		data, ok := all[r.PathValue("name")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func (s *releaseServer) env() []string {
	return []string{"ST_RELEASES_API=" + s.URL}
}

// fakeRelease is a stand-in for a released st binary: a script that reports
// version for 'version --json'.
func fakeRelease(version string) []byte {
	return []byte(fmt.Sprintf("#!/bin/sh\necho '{\"version\": \"%s\", \"release\": true}'\n", version))
}

func thisPlatformAsset() string {
	return "st-" + runtime.GOOS + "-" + runtime.GOARCH
}

// installedSt copies a release build of st stamped with version into a
// folder of its own and reaches it through a link, as ~/.local/bin/st often
// is. It returns the link and the file.
func installedSt(t *testing.T, version string) (link, file string) {
	t.Helper()
	data, err := os.ReadFile(stampedBinary(t, version))
	if err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(t.TempDir(), "st")
	if err := os.WriteFile(file, data, 0o755); err != nil {
		t.Fatal(err)
	}
	link = filepath.Join(t.TempDir(), "st")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	return link, file
}

func sameFile(t *testing.T, path string, want []byte) bool {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(got) == string(want)
}

func TestUpdateInstallsANewerRelease(t *testing.T) {
	link, file := installedSt(t, "v0.4.2")
	old, _ := os.ReadFile(file)
	newBinary := fakeRelease("v0.5.0")
	gh := newReleaseServer(t, "v0.5.0", map[string][]byte{thisPlatformAsset(): newBinary, "st-plan9-mips": []byte("other")}, 0)

	out, err := runBinary(t, link, t.TempDir(), gh.env(), "update")
	if err != nil {
		t.Fatalf("st update: %v\n%s", err, out)
	}

	assertContains(t, out, "v0.5.0")
	if !sameFile(t, file, newBinary) {
		t.Error("the binary behind the link is not the new release")
	}
	if !sameFile(t, file+".prev", old) {
		t.Error("the previous binary was not kept as st.prev")
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was replaced: %v", err)
	}
	assertDirHolds(t, filepath.Dir(file), "st", "st.prev")
}

func TestUpdateWhenAlreadyTheNewest(t *testing.T) {
	for _, latest := range []string{"v0.4.2", "v0.4.1"} {
		t.Run(latest, func(t *testing.T) {
			link, file := installedSt(t, "v0.4.2")
			old, _ := os.ReadFile(file)
			gh := newReleaseServer(t, latest, map[string][]byte{thisPlatformAsset(): fakeRelease(latest)}, 0)

			out, err := runBinary(t, link, t.TempDir(), gh.env(), "update")
			if err != nil {
				t.Fatalf("st update: %v\n%s", err, out)
			}

			assertContains(t, out, "up to date")
			if !sameFile(t, file, old) {
				t.Error("the binary was changed")
			}
			assertDirHolds(t, filepath.Dir(file), "st")
		})
	}
}

func TestUpdateRefusesWhatItCannotTrust(t *testing.T) {
	asset := thisPlatformAsset()
	cases := map[string]struct {
		assets map[string][]byte
		status int
		want   string
	}{
		"GitHub refuses": {status: http.StatusForbidden, want: "403"},
		"no binary for this platform": {
			assets: map[string][]byte{"st-plan9-mips": []byte("other")},
			want:   asset,
		},
		"a binary that reports another version": {
			assets: map[string][]byte{asset: fakeRelease("v0.4.9")},
			want:   "v0.4.9",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			link, file := installedSt(t, "v0.4.2")
			old, _ := os.ReadFile(file)
			gh := newReleaseServer(t, "v0.5.0", c.assets, c.status)

			out, err := runBinary(t, link, t.TempDir(), gh.env(), "update")

			if err == nil {
				t.Fatalf("st update succeeded:\n%s", out)
			}
			assertContains(t, out, c.want)
			if !sameFile(t, file, old) {
				t.Error("the binary was changed")
			}
			assertDirHolds(t, filepath.Dir(file), "st")
		})
	}
}

func TestUpdateRefusesABuildFromSource(t *testing.T) {
	gh := newReleaseServer(t, "v0.5.0", map[string][]byte{thisPlatformAsset(): fakeRelease("v0.5.0")}, 0)
	// stBinary is shared by every test: copy it, in case update writes.
	data, err := os.ReadFile(stBinary)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "st")
	if err := os.WriteFile(binary, data, 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runBinary(t, binary, t.TempDir(), gh.env(), "update")

	if err == nil {
		t.Fatalf("st update succeeded in a dev build:\n%s", out)
	}
	assertContains(t, out, "dev")
	assertContains(t, out, "rebuild")
	if n := gh.requests.Load(); n != 0 {
		t.Errorf("a dev build made %d requests to GitHub", n)
	}
	if !sameFile(t, binary, data) {
		t.Error("the binary was changed")
	}
}

type updateCheck struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
}

func TestUpdateCheckReportsWithoutInstalling(t *testing.T) {
	link, file := installedSt(t, "v0.4.2")
	old, _ := os.ReadFile(file)
	gh := newReleaseServer(t, "v0.5.0", map[string][]byte{thisPlatformAsset(): fakeRelease("v0.5.0")}, 0)

	out, err := runBinary(t, link, t.TempDir(), gh.env(), "update", "--check")
	if err != nil {
		t.Fatalf("st update --check: %v\n%s", err, out)
	}
	assertContains(t, out, "v0.4.2")
	assertContains(t, out, "v0.5.0")

	out, err = runBinary(t, link, t.TempDir(), gh.env(), "update", "--check", "--json")
	if err != nil {
		t.Fatalf("st update --check --json: %v\n%s", err, out)
	}
	var got updateCheck
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("st update --check --json printed %q: %v", out, err)
	}
	want := updateCheck{Current: "v0.4.2", Latest: "v0.5.0", Available: true, URL: "https://github.com/cpave3/staccato/releases/tag/v0.5.0"}
	if got != want {
		t.Errorf("st update --check --json = %+v, want %+v", got, want)
	}

	if !sameFile(t, file, old) {
		t.Error("--check changed the binary")
	}
	assertDirHolds(t, filepath.Dir(file), "st")
	if n := gh.requests.Load(); n != 2 {
		t.Errorf("--check made %d requests, want one each (nothing downloaded)", n)
	}
}

func TestUpdateCheckWorksInABuildFromSource(t *testing.T) {
	gh := newReleaseServer(t, "v0.5.0", map[string][]byte{thisPlatformAsset(): fakeRelease("v0.5.0")}, 0)

	out, err := runBinary(t, stBinary, t.TempDir(), gh.env(), "update", "--check", "--json")
	if err != nil {
		t.Fatalf("st update --check --json: %v\n%s", err, out)
	}
	var got updateCheck
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("st update --check --json printed %q: %v", out, err)
	}
	if got.Current != "dev" || got.Latest != "v0.5.0" || got.Available {
		t.Errorf("st update --check --json = %+v, want dev, v0.5.0, not available", got)
	}
}

func assertDirHolds(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != strings.Join(names, ",") {
		t.Errorf("%s holds %v, want %v", dir, got, names)
	}
}
