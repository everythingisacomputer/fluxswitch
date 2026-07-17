package main

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
)

// switchInteractive shows an arrow-key picker over installed and available
// versions, then switches to the selection.
func switchInteractive(home *Home) error {
	installed, err := home.InstalledVersions()
	if err != nil {
		return err
	}
	installedSet := make(map[string]bool, len(installed))
	for _, v := range installed {
		installedSet[v] = true
	}

	// Start from remote releases so new versions are offered; fall back to
	// installed-only when offline.
	versions, err := AvailableVersions(30)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not reach GitHub (%v), showing installed versions only\n", err)
	}
	remoteSet := make(map[string]bool, len(versions))
	for _, v := range versions {
		remoteSet[v] = true
	}
	// Keep installed versions visible even if they fell out of the recent
	// releases window.
	for _, v := range installed {
		if !remoteSet[v] {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return fmt.Errorf("no versions available - check your network connection")
	}

	current, _ := home.CurrentVersion()

	type item struct {
		Version string
		Status  string
	}
	items := make([]item, 0, len(versions))
	cursor := 0
	for i, v := range versions {
		status := ""
		switch {
		case v == current:
			status = "(current)"
			cursor = i
		case installedSet[v]:
			status = "(installed)"
		}
		items = append(items, item{Version: v, Status: status})
	}

	prompt := promptui.Select{
		Label:     "Select flux version",
		Items:     items,
		CursorPos: cursor,
		Size:      12,
		Templates: &promptui.SelectTemplates{
			Active:   `{{ "▸" | cyan }} {{ .Version | cyan }} {{ .Status | faint }}`,
			Inactive: `  {{ .Version }} {{ .Status | faint }}`,
			Selected: `{{ "✔" | green }} flux {{ .Version }}`,
		},
		Searcher: func(input string, index int) bool {
			return fuzzyMatch(items[index].Version, input)
		},
		StartInSearchMode: false,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}
	return switchTo(home, items[idx].Version)
}

// fuzzyMatch reports whether the characters of input appear in order in s.
func fuzzyMatch(s, input string) bool {
	j := 0
	for i := 0; i < len(s) && j < len(input); i++ {
		if s[i] == input[j] {
			j++
		}
	}
	return j == len(input)
}
