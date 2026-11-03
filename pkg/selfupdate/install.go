package selfupdate

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// sumsAsset is the release's list of SHA-256 checksums, in sha256sum's form.
const sumsAsset = "SHA256SUMS"

// AssetName is the release file of the binary for a platform.
func (c Client) AssetName(goos, goarch string) string {
	return c.Asset + "-" + goos + "-" + goarch
}

// Install puts the release's binary for the platform this runs on in place
// of exe, following a link to the file. The binary is downloaded beside exe,
// checked against the release's SHA256SUMS, then handed to verify (which
// runs it, say) before it is renamed over exe: a process still running the
// old file goes on running it, and macOS, which kills a signed binary whose
// file changed, sees a new file. The old binary is kept as <exe>.prev.
// Nothing is changed when any step fails.
func (c Client) Install(ctx context.Context, rel Release, exe string, verify func(ctx context.Context, binary string) error) error {
	asset := c.AssetName(runtime.GOOS, runtime.GOARCH)
	target, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	url, ok := rel.Assets[asset]
	if !ok {
		return fmt.Errorf("release %s has no %s", rel.Version, asset)
	}
	sumsURL, ok := rel.Assets[sumsAsset]
	if !ok {
		return fmt.Errorf("release %s has no %s to check its checksum against", rel.Version, sumsAsset)
	}
	want, err := c.checksum(ctx, sumsURL, asset)
	if err != nil {
		return fmt.Errorf("release %s: %w", rel.Version, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s, where the running binary is: %w", filepath.Dir(target), err)
	}
	defer os.Remove(tmp.Name())
	got, err := c.download(ctx, url, tmp)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("downloading %s of %s: %w", asset, rel.Version, err)
	}
	if got != want {
		return fmt.Errorf("%s of %s does not match its checksum", asset, rel.Version)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if err := verify(ctx, tmp.Name()); err != nil {
		return fmt.Errorf("%s of %s: %w", asset, rel.Version, err)
	}

	prev := target + ".prev"
	_ = os.Remove(prev)
	if err := os.Link(target, prev); err != nil {
		return fmt.Errorf("keeping the running binary as %s: %w", prev, err)
	}
	return os.Rename(tmp.Name(), target)
}

// checksum is asset's SHA-256 in the release's list.
func (c Client) checksum(ctx context.Context, url, asset string) (string, error) {
	body, err := c.get(ctx, url)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", sumsAsset, err)
	}
	defer body.Close()
	lines := bufio.NewScanner(io.LimitReader(body, 1<<20))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := lines.Err(); err != nil {
		return "", fmt.Errorf("reading %s: %w", sumsAsset, err)
	}
	return "", fmt.Errorf("%s has no checksum for %s", sumsAsset, asset)
}

// download writes the file at url to w and returns its SHA-256.
func (c Client) download(ctx context.Context, url string, w io.Writer) (string, error) {
	body, err := c.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(w, hash), body); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (c Client) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s answered %s", url, resp.Status)
	}
	return resp.Body, nil
}
