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
)

// Install downloads the flux release tarball for this OS/arch, verifies it
// against the release's published SHA-256 checksums, and extracts the flux
// binary into the version directory.
func Install(home *Home, version string) error {
	asset := fmt.Sprintf("flux_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)

	expected, err := fetchChecksum(version, asset)
	if err != nil {
		return err
	}

	url := fmt.Sprintf(downloadURL, version, asset)
	resp, err := httpClient.Get(url)
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
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != expected {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s (corrupted download, try again)", asset, got, expected)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if err := os.MkdirAll(home.versionDir(version), 0o755); err != nil {
		return err
	}
	if err := extractFlux(tmp, home.binaryPath(version)); err != nil {
		// Don't leave a half-installed version behind.
		os.RemoveAll(home.versionDir(version))
		return fmt.Errorf("extracting flux %s: %w", version, err)
	}
	return nil
}

// fetchChecksum returns the published SHA-256 hash for the named release
// asset from the release's checksums file.
func fetchChecksum(version, asset string) (string, error) {
	url := fmt.Sprintf(downloadURL, version, fmt.Sprintf(checksumFile, version))
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: got %s", url, resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no checksum for %s in %s", asset, url)
}

// extractFlux pulls the "flux" binary out of a gzipped tarball stream and
// writes it to dest atomically.
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
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		if err := f.Close(); err != nil {
			os.Remove(tmp)
			return err
		}
		return os.Rename(tmp, dest)
	}
}
