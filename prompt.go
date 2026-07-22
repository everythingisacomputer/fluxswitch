package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	ansiReset = "\x1b[0m"
	ansiDim   = "\x1b[2m"
	ansiCyan  = "\x1b[36m"

	pickerHeight = 12
)

// switchInteractive shows an arrow-key picker over installed and available
// versions, then switches to the selection. Uninstalled versions are dimmed;
// 'd' downloads the highlighted version without leaving the picker, and
// selecting an uninstalled version asks for confirmation first.
func switchInteractive(home *Home) error {
	installed, err := home.InstalledVersions()
	if err != nil {
		return err
	}
	installedSet := make(map[string]bool, len(installed))
	for _, v := range installed {
		installedSet[v] = true
	}

	// Remote releases fill out the rest of the list; fall back to
	// installed-only when offline.
	remote, err := AvailableVersions(30)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not reach GitHub (%v), showing installed versions only\n", err)
	}
	if len(installed) == 0 && len(remote) == 0 {
		return fmt.Errorf("no versions available - check your network connection")
	}

	current, _ := home.CurrentVersion()

	// Installed versions come first (newest first), then the remaining
	// remote versions newest to oldest, greyed out until installed.
	items := make([]pickItem, 0, len(installed)+len(remote))
	for _, v := range installed {
		items = append(items, pickItem{version: v, installed: true, current: v == current})
	}
	for _, v := range remote {
		if !installedSet[v] {
			items = append(items, pickItem{version: v})
		}
	}

	m := newPickerModel(home, items)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	picker := final.(pickerModel)
	if picker.choice == "" {
		return nil
	}
	return switchTo(home, picker.choice)
}

type pickItem struct {
	version     string
	installed   bool
	current     bool
	downloading bool
}

func (it pickItem) status() string {
	switch {
	case it.downloading:
		return "(downloading...)"
	case it.current:
		return "(current)"
	case it.installed:
		return "(installed)"
	}
	return ""
}

// installDoneMsg reports the result of a background download started with 'd'.
type installDoneMsg struct {
	version string
	err     error
}

type pickerModel struct {
	home  *Home
	items []pickItem

	visible []int // indexes into items that match the search query
	cursor  int   // position within visible
	offset  int   // scroll offset within visible

	searching  bool
	query      string
	confirming int // items index pending install confirmation, -1 if none

	status   string // transient message shown under the list
	choice   string // version to switch to once the picker exits
	quitting bool
}

func newPickerModel(home *Home, items []pickItem) pickerModel {
	m := pickerModel{home: home, items: items, confirming: -1}
	m.refilter()
	for i, idx := range m.visible {
		if items[idx].current {
			m.cursor = i
		}
	}
	m.scrollToCursor()
	return m
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case installDoneMsg:
		for i := range m.items {
			if m.items[i].version == msg.version {
				m.items[i].downloading = false
				m.items[i].installed = msg.err == nil
			}
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("download of flux %s failed: %v", msg.version, msg.err)
		} else {
			m.status = fmt.Sprintf("flux %s downloaded", msg.version)
		}
		return m, nil

	case tea.KeyMsg:
		if key := msg.String(); key == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if m.confirming >= 0 {
			return m.updateConfirm(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateBrowse(msg)
	}
	return m, nil
}

func (m pickerModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.choice = m.items[m.confirming].version
		m.quitting = true
		return m, tea.Quit
	default: // anything else declines
		m.confirming = -1
		return m, nil
	}
}

func (m pickerModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.query = ""
		m.refilter()
		return m, nil
	case "enter":
		return m.selectCursor()
	case "up", "down":
		m.moveCursor(msg.String())
		return m, nil
	case "backspace":
		if m.query != "" {
			m.query = m.query[:len(m.query)-1]
			m.refilter()
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.query += string(msg.Runes)
			m.refilter()
		}
		return m, nil
	}
}

func (m pickerModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	case "up", "k", "down", "j":
		m.moveCursor(msg.String())
		return m, nil
	case "/":
		m.searching = true
		m.status = ""
		return m, nil
	case "d":
		return m.downloadCursor()
	case "u":
		return m.uninstallCursor()
	case "enter":
		return m.selectCursor()
	}
	return m, nil
}

// selectCursor acts on enter: switch to installed versions immediately, ask
// before installing uninstalled ones.
func (m pickerModel) selectCursor() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	idx := m.visible[m.cursor]
	it := m.items[idx]
	if it.downloading {
		m.status = fmt.Sprintf("flux %s is still downloading", it.version)
		return m, nil
	}
	if !it.installed {
		m.confirming = idx
		return m, nil
	}
	m.choice = it.version
	m.quitting = true
	return m, tea.Quit
}

// downloadCursor acts on 'd': install the highlighted version in the
// background while the picker stays open.
func (m pickerModel) downloadCursor() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	idx := m.visible[m.cursor]
	it := m.items[idx]
	if it.installed {
		m.status = fmt.Sprintf("flux %s is already installed", it.version)
		return m, nil
	}
	if it.downloading {
		return m, nil
	}
	m.items[idx].downloading = true
	m.status = ""
	home, version := m.home, it.version
	return m, func() tea.Msg {
		return installDoneMsg{version: version, err: Install(home, version)}
	}
}

// uninstallCursor acts on 'u': remove the highlighted version from disk. The
// active version is protected so the flux symlink can't be left dangling.
func (m pickerModel) uninstallCursor() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	idx := m.visible[m.cursor]
	it := m.items[idx]
	switch {
	case it.downloading:
		m.status = fmt.Sprintf("flux %s is still downloading", it.version)
	case !it.installed:
		m.status = fmt.Sprintf("flux %s is not installed", it.version)
	case it.current:
		m.status = fmt.Sprintf("flux %s is the active version - switch away from it first", it.version)
	default:
		if err := m.home.Uninstall(it.version); err != nil {
			m.status = fmt.Sprintf("uninstalling flux %s failed: %v", it.version, err)
		} else {
			m.items[idx].installed = false
			m.status = fmt.Sprintf("flux %s uninstalled", it.version)
		}
	}
	return m, nil
}

func (m *pickerModel) moveCursor(key string) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
	}
	m.scrollToCursor()
}

func (m *pickerModel) refilter() {
	m.visible = m.visible[:0]
	for i, it := range m.items {
		if fuzzyMatch(it.version, m.query) {
			m.visible = append(m.visible, i)
		}
	}
	m.cursor = 0
	m.offset = 0
}

func (m *pickerModel) scrollToCursor() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+pickerHeight {
		m.offset = m.cursor - pickerHeight + 1
	}
}

func (m pickerModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder

	if m.searching {
		fmt.Fprintf(&b, "Select flux version %s/%s█%s\n", ansiCyan, m.query, ansiReset)
	} else {
		b.WriteString("Select flux version:\n")
	}

	if len(m.visible) == 0 {
		fmt.Fprintf(&b, "  %sno matching versions%s\n", ansiDim, ansiReset)
	}
	end := min(m.offset+pickerHeight, len(m.visible))
	for i := m.offset; i < end; i++ {
		it := m.items[m.visible[i]]

		version := it.version
		if !it.installed {
			version = ansiDim + version + ansiReset
		}
		status := ""
		if s := it.status(); s != "" {
			status = " " + ansiDim + s + ansiReset
		}

		if i == m.cursor {
			cursorVersion := ansiCyan + it.version + ansiReset
			if !it.installed {
				cursorVersion = ansiDim + it.version + ansiReset
			}
			fmt.Fprintf(&b, "%s▸%s %s%s\n", ansiCyan, ansiReset, cursorVersion, status)
		} else {
			fmt.Fprintf(&b, "  %s%s\n", version, status)
		}
	}
	if end < len(m.visible) {
		fmt.Fprintf(&b, "%s  ↓ %d more%s\n", ansiDim, len(m.visible)-end, ansiReset)
	}

	switch {
	case m.confirming >= 0:
		fmt.Fprintf(&b, "\nflux %s isn't installed - install and switch to it? [y/N]\n", m.items[m.confirming].version)
	case m.status != "":
		fmt.Fprintf(&b, "\n%s\n", m.status)
	default:
		fmt.Fprintf(&b, "\n%s↑/↓ move · enter switch · d download · u uninstall · / search · q quit%s\n", ansiDim, ansiReset)
	}
	return b.String()
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
