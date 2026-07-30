package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

const (
	downloadURL  = "https://github.com/fluxcd/flux2/releases/download/v%s/%s"
	checksumFile = "flux_%s_checksums.txt"

	// Size ceilings, generous multiples of real flux releases (~75 MB
	// tarball). They bound disk/memory use if a download or archive is
	// ever malformed or malicious.
	maxTarballBytes  = 512 << 20
	maxBinaryBytes   = 512 << 20
	maxChecksumBytes = 1 << 20
)

// Install downloads the flux release tarball for this OS/arch, verifies it
// against the release's published SHA-256 checksums, and extracts the flux
// binary into the version directory.
func Install(home *Home, version string) error {
	if !isValidVersion(version) {
		return fmt.Errorf("invalid version %q", version)
	}
	asset := fmt.Sprintf("flux_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)

	expected, err := fetchChecksum(version, asset)
	if err != nil {
		return err
	}

	url := fmt.Sprintf(downloadURL, version, asset)
	// Host is the github.com constant; version/asset are regex-validated
	// path segments that cannot contain separators or userinfo.
	resp, err := downloadClient.Get(url) // #nosec G704
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("flux %s has no release for %s/%s (checked %s)", version, runtime.GOOS, runtime.GOARCH, url)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: got %s", url, resp.Status)
	}

	// Buffer the tarball to a temp file so the whole download can be
	// checksummed before anything is extracted.
	tmp, err := os.CreateTemp("", "fluxswitch-*.tar.gz")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxTarballBytes+1))
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	if n > maxTarballBytes {
		return fmt.Errorf("downloading %s: exceeds %d byte limit", url, int64(maxTarballBytes))
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != expected {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s (corrupted download, try again)", asset, got, expected)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if err := os.MkdirAll(home.versionDir(version), 0o750); err != nil {
		return err
	}
	if err := extractFlux(tmp, home.binaryPath(version)); err != nil {
		// Best-effort: don't leave a half-installed version behind.
		_ = os.RemoveAll(home.versionDir(version))
		return fmt.Errorf("extracting flux %s: %w", version, err)
	}
	return nil
}

// fetchChecksum returns the published SHA-256 hash for the named release
// asset from the release's checksums file.
func fetchChecksum(version, asset string) (string, error) {
	url := fmt.Sprintf(downloadURL, version, fmt.Sprintf(checksumFile, version))
	// Same as the tarball fetch: fixed host, validated version segment.
	resp, err := apiClient.Get(url) // #nosec G704
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: got %s", url, resp.Status)
	}

	sum, err := findChecksum(io.LimitReader(resp.Body, maxChecksumBytes), asset)
	if err != nil {
		return "", fmt.Errorf("%w in %s", err, url)
	}
	return sum, nil
}

// findChecksum scans sha256sum-format lines ("<hex>  <name>", with an
// optional binary-mode "*" prefix on the name) for the given asset.
func findChecksum(r io.Reader, asset string) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no checksum for %s", asset)
}

// extractFlux pulls the "flux" binary out of a gzipped tarball stream and
// writes it to dest atomically. The stream's integrity has already been
// verified against the release checksum by the caller.
func extractFlux(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("no flux binary found in archive")
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || hdr.FileInfo().Name() != "flux" {
			continue
		}

		tmp := dest + ".tmp"
		// 0755 so the extracted flux binary is executable; the path is
		// derived from a validated version string under ~/.fluxswitch.
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) // #nosec G302 G304
		if err != nil {
			return err
		}
		n, err := io.Copy(f, io.LimitReader(tr, maxBinaryBytes+1))
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
		if n > maxBinaryBytes {
			_ = f.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("flux binary exceeds %d byte limit", int64(maxBinaryBytes))
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return os.Rename(tmp, dest)
	}
}
