package main

import (
	"strings"
	"testing"
)

func TestIsValidVersion(t *testing.T) {
	valid := []string{"2.9.3", "0.0.1", "2.0.0-rc.1", "10.20.30", "2.0.0-beta.2"}
	for _, v := range valid {
		if !isValidVersion(v) {
			t.Errorf("isValidVersion(%q) = false, want true", v)
		}
	}

	invalid := []string{
		"", ".", "..", "../2.9.3", "2.9.3/..", "2.9", "v2.9.3",
		"2.9.3 ", " 2.9.3", "2.9.3/", "2.9.3\n", "2.9.3;rm -rf",
		"2.9.3-", "..2.9.3", "latest",
	}
	for _, v := range invalid {
		if isValidVersion(v) {
			t.Errorf("isValidVersion(%q) = true, want false", v)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int // sign only
	}{
		{"2.9.3", "2.9.2", 1},
		{"2.9.2", "2.9.3", -1},
		{"2.9.3", "2.9.3", 0},
		{"2.10.0", "2.9.9", 1},  // numeric, not lexicographic
		{"10.0.0", "9.99.99", 1},
		{"2.9.3", "2.9", 1},
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		switch {
		case c.want > 0 && got <= 0,
			c.want < 0 && got >= 0,
			c.want == 0 && got != 0:
			t.Errorf("compareVersions(%q, %q) = %d, want sign %d", c.a, c.b, got, c.want)
		}
	}
}

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		s, input string
		want     bool
	}{
		{"2.9.3", "", true},
		{"2.9.3", "2.9.3", true},
		{"2.9.3", "293", true},
		{"2.9.3", "29.", true}, // 2, 9, then the dot before the 3
		{"2.9.3", "39", false}, // order matters
		{"2.9.3", "932", false},
		{"2.9.3", "2.9.3.1", false},
	}
	for _, c := range cases {
		if got := fuzzyMatch(c.s, c.input); got != c.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", c.s, c.input, got, c.want)
		}
	}
}

func TestFindChecksum(t *testing.T) {
	checksums := strings.Join([]string{
		"aaaa  flux_2.9.3_darwin_amd64.tar.gz",
		"bbbb  flux_2.9.3_darwin_arm64.tar.gz",
		"cccc  *flux_2.9.3_linux_amd64.tar.gz", // binary-mode marker
		"malformed line without enough or with too many fields here",
	}, "\n")

	got, err := findChecksum(strings.NewReader(checksums), "flux_2.9.3_darwin_arm64.tar.gz")
	if err != nil || got != "bbbb" {
		t.Errorf("findChecksum(darwin_arm64) = %q, %v; want \"bbbb\", nil", got, err)
	}

	got, err = findChecksum(strings.NewReader(checksums), "flux_2.9.3_linux_amd64.tar.gz")
	if err != nil || got != "cccc" {
		t.Errorf("findChecksum(*linux_amd64) = %q, %v; want \"cccc\", nil", got, err)
	}

	if _, err = findChecksum(strings.NewReader(checksums), "flux_2.9.3_windows_amd64.zip"); err == nil {
		t.Error("findChecksum(missing asset) = nil error, want error")
	}
}
