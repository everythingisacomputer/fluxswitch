package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Home manages the ~/.fluxswitch directory layout:
//
//	~/.fluxswitch/versions/<version>/flux   installed binaries
//	~/.fluxswitch/bin/flux                  symlink to the active version
type Home struct {
	root    string
	binLink string
}

func NewHome() (*Home, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(userHome, ".fluxswitch")

	binLink := os.Getenv("FLUXSWITCH_BIN")
	if binLink == "" {
		binLink = filepath.Join(root, "bin", "flux")
	}

	if err := os.MkdirAll(filepath.Join(root, "versions"), 0o750); err != nil {
		return nil, err
	}
	// FLUXSWITCH_BIN is the user's own opt-in override for where the flux
	// symlink lives (documented feature, same trust model as GOBIN).
	if err := os.MkdirAll(filepath.Dir(binLink), 0o750); err != nil { // #nosec G703 G301
		return nil, err
	}
	return &Home{root: root, binLink: binLink}, nil
}

func (h *Home) BinLink() string { return h.binLink }

func (h *Home) versionDir(version string) string {
	return filepath.Join(h.root, "versions", version)
}

func (h *Home) binaryPath(version string) string {
	return filepath.Join(h.versionDir(version), "flux")
}

func (h *Home) IsInstalled(version string) bool {
	info, err := os.Stat(h.binaryPath(version))
	return err == nil && info.Mode().IsRegular()
}

// InstalledVersions returns installed versions, newest first.
func (h *Home) InstalledVersions() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(h.root, "versions"))
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && h.IsInstalled(e.Name()) {
			versions = append(versions, e.Name())
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})
	return versions, nil
}

// CurrentVersion resolves the active symlink back to a version string.
func (h *Home) CurrentVersion() (string, error) {
	target, err := os.Readlink(h.binLink)
	if err != nil {
		return "", err
	}
	// target looks like ~/.fluxswitch/versions/<version>/flux
	return filepath.Base(filepath.Dir(target)), nil
}

// Uninstall removes an installed version from disk. The version is
// re-validated here because this is the only destructive path: it must never
// resolve outside the versions directory.
func (h *Home) Uninstall(version string) error {
	if !isValidVersion(version) {
		return fmt.Errorf("invalid version %q", version)
	}
	if !h.IsInstalled(version) {
		return fmt.Errorf("version %s is not installed", version)
	}
	return os.RemoveAll(h.versionDir(version))
}

// Activate points the bin symlink at the given installed version.
func (h *Home) Activate(version string) error {
	if !h.IsInstalled(version) {
		return fmt.Errorf("version %s is not installed", version)
	}
	tmp := h.binLink + ".tmp"
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(h.binaryPath(version), tmp); err != nil {
		return err
	}
	return os.Rename(tmp, h.binLink)
}

// compareVersions compares dotted numeric versions like "2.10.1" and "2.9.0".
// Returns >0 if a is newer than b, <0 if older, 0 if equal. Non-numeric
// segments fall back to string comparison.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		an, aerr := strconv.Atoi(av)
		bn, berr := strconv.Atoi(bv)
		if aerr == nil && berr == nil {
			if an != bn {
				return an - bn
			}
			continue
		}
		if c := strings.Compare(av, bv); c != 0 {
			return c
		}
	}
	return 0
}
