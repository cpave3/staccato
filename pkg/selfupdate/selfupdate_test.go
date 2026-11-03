package selfupdate_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cpave3/staccato/pkg/selfupdate"
)

// fakeGitHub serves a latest release and its assets as GitHub does.
type fakeGitHub struct {
	*httptest.Server
	tag    string
	assets map[string][]byte
	sums   string // SHA256SUMS; computed from assets when ""
	status int    // of the latest-release call; 200 when 0
}

func newFakeGitHub(t *testing.T, tag string, assets map[string][]byte) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{tag: tag, assets: assets}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/cpave3/staccato/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		list := ""
		for name := range f.assetsWithSums() {
			if list != "" {
				list += ","
			}
			list += fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, name, f.URL+"/download/"+name)
		}
		fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://github.com/cpave3/staccato/releases/tag/%s","draft":false,"prerelease":false,"assets":[%s]}`, f.tag, f.tag, list)
	})
	mux.HandleFunc("GET /download/{name}", func(w http.ResponseWriter, r *http.Request) {
		data, ok := f.assetsWithSums()[r.PathValue("name")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeGitHub) assetsWithSums() map[string][]byte {
	out := map[string][]byte{}
	sums := f.sums
	for name, data := range f.assets {
		out[name] = data
		if f.sums == "" {
			sum := sha256.Sum256(data)
			sums += hex.EncodeToString(sum[:]) + "  " + name + "\n"
		}
	}
	out["SHA256SUMS"] = []byte(sums)
	return out
}

func (f *fakeGitHub) client() selfupdate.Client {
	return selfupdate.Client{API: f.URL, Repo: "cpave3/staccato", Asset: "st", HTTP: f.Client()}
}

func TestLatestIsTheNewestStableRelease(t *testing.T) {
	gh := newFakeGitHub(t, "v1.2.0", map[string][]byte{"st-linux-amd64": []byte("bin")})

	rel, err := gh.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if rel.Version != "v1.2.0" || rel.URL != "https://github.com/cpave3/staccato/releases/tag/v1.2.0" {
		t.Errorf("release = %+v", rel)
	}
	if rel.Assets["st-linux-amd64"] != gh.URL+"/download/st-linux-amd64" {
		t.Errorf("assets = %v", rel.Assets)
	}
}

func TestLatestSaysWhenGitHubRefuses(t *testing.T) {
	gh := newFakeGitHub(t, "v1.2.0", nil)
	gh.status = http.StatusForbidden

	if _, err := gh.client().Latest(context.Background()); err == nil {
		t.Error("no error for a 403")
	}
}

// installed is a running binary in a folder of its own, reached through a
// link as ~/.local/bin/st often is. It returns the link and the file.
func installed(t *testing.T, content string) (link, file string) {
	t.Helper()
	dir := t.TempDir()
	file = filepath.Join(dir, "st")
	if err := os.WriteFile(file, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	link = filepath.Join(t.TempDir(), "st")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	return link, file
}

func accept(context.Context, string) error { return nil }

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallPutsTheReleaseInPlaceOfTheRunningBinary(t *testing.T) {
	asset := selfupdate.Client{Asset: "st"}.AssetName(runtime.GOOS, runtime.GOARCH)
	gh := newFakeGitHub(t, "v1.2.0", map[string][]byte{asset: []byte("new"), "st-other-os": []byte("other")})
	link, file := installed(t, "old")
	rel, err := gh.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var verified string
	verify := func(_ context.Context, binary string) error {
		verified = read(t, binary)
		return nil
	}

	if err := gh.client().Install(context.Background(), rel, link, verify); err != nil {
		t.Fatal(err)
	}

	if verified != "new" {
		t.Errorf("verified %q before installing, want the new binary", verified)
	}
	if got := read(t, file); got != "new" {
		t.Errorf("binary = %q, want new", got)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link to the binary was replaced: %v", err)
	}
	if info, _ := os.Stat(file); info.Mode().Perm()&0o111 == 0 {
		t.Errorf("new binary is not executable: %v", info.Mode())
	}
	if got := read(t, file+".prev"); got != "old" {
		t.Errorf("previous binary = %q, want old", got)
	}
	assertOnly(t, filepath.Dir(file), "st", "st.prev")
}

func TestInstallRefusesWhatItCannotTrust(t *testing.T) {
	asset := selfupdate.Client{Asset: "st"}.AssetName(runtime.GOOS, runtime.GOARCH)
	cases := map[string]struct {
		assets map[string][]byte
		sums   string
		verify func(context.Context, string) error
		want   string
	}{
		"a checksum that does not match": {
			assets: map[string][]byte{asset: []byte("new")},
			sums:   strings.Repeat("0", 64) + "  " + asset + "\n",
			verify: accept, want: "checksum",
		},
		"no checksum for the binary": {
			assets: map[string][]byte{asset: []byte("new")},
			sums:   strings.Repeat("0", 64) + "  st-other-os\n",
			verify: accept, want: "checksum",
		},
		"no binary for this platform": {
			assets: map[string][]byte{"st-other-os": []byte("other")},
			verify: accept, want: asset,
		},
		"a binary that fails its check": {
			assets: map[string][]byte{asset: []byte("new")},
			verify: func(context.Context, string) error { return fmt.Errorf("says v0.9.0") },
			want:   "says v0.9.0",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			gh := newFakeGitHub(t, "v1.2.0", c.assets)
			gh.sums = c.sums
			link, file := installed(t, "old")
			rel, err := gh.client().Latest(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			err = gh.client().Install(context.Background(), rel, link, c.verify)

			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one naming %q", err, c.want)
			}
			if got := read(t, file); got != "old" {
				t.Errorf("binary = %q, want it left alone", got)
			}
			assertOnly(t, filepath.Dir(file), "st")
		})
	}
}

func assertOnly(t *testing.T, dir string, names ...string) {
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

func TestLatestSaysWhenNothingIsPublishedYet(t *testing.T) {
	gh := newFakeGitHub(t, "", nil)
	gh.status = http.StatusNotFound

	if _, err := gh.client().Latest(context.Background()); !errors.Is(err, selfupdate.ErrNoRelease) {
		t.Errorf("err = %v, want ErrNoRelease", err)
	}
}

func TestAssetNameUsesTheClientsPrefix(t *testing.T) {
	if got := (selfupdate.Client{Asset: "st"}).AssetName("linux", "arm64"); got != "st-linux-arm64" {
		t.Errorf("AssetName = %q, want st-linux-arm64", got)
	}
}

func TestInstallSaysWhichDirectoryItCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to any directory")
	}
	asset := selfupdate.Client{Asset: "st"}.AssetName(runtime.GOOS, runtime.GOARCH)
	gh := newFakeGitHub(t, "v1.2.0", map[string][]byte{asset: []byte("new")})
	link, file := installed(t, "old")
	dir := filepath.Dir(file)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	rel, err := gh.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = gh.client().Install(context.Background(), rel, link, accept)

	if err == nil || !strings.Contains(err.Error(), dir) || !strings.Contains(err.Error(), "cannot write") {
		t.Errorf("err = %v, want one saying it cannot write to %s", err, dir)
	}
	if got := read(t, file); got != "old" {
		t.Errorf("binary = %q, want it left alone", got)
	}
	assertOnly(t, dir, "st")
}
