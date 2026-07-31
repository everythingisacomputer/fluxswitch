// fluxswitch lets you switch between versions of the Flux CLI (fluxcd/flux2),
// in the same spirit as tfswitch for Terraform.
//
// Usage:
//
//	fluxswitch            # interactive picker (installed + available versions)
//	fluxswitch 2.3.0      # switch to a specific version, downloading it if needed
//	fluxswitch --list     # list installed versions
//	fluxswitch --latest   # switch to the latest release
package main

import (
	"fmt"
	"os"
	"strings"
)

// version is stamped by GoReleaser at release time.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fluxswitch:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	home, err := NewHome()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return switchInteractive(home)
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "-v", "--version", "version":
		fmt.Println("fluxswitch", version)
		return nil
	case "-l", "--list", "list":
		return listInstalled(home)
	case "--latest":
		version, err := LatestVersion()
		if err != nil {
			return err
		}
		return switchTo(home, version)
	default:
		version := strings.TrimPrefix(args[0], "v")
		if !isValidVersion(version) {
			return fmt.Errorf("%q doesn't look like a flux version (expected something like 2.9.3)", args[0])
		}
		return switchTo(home, version)
	}
}

func printUsage() {
	fmt.Print(`fluxswitch - switch between Flux CLI versions

Usage:
  fluxswitch            interactive version picker
  fluxswitch <version>  switch to a version (e.g. 2.3.0), downloading if needed
  fluxswitch --latest   switch to the latest Flux release
  fluxswitch --list     list installed versions
  fluxswitch --version  print the fluxswitch version
  fluxswitch --help     show this help

Versions are stored in ~/.fluxswitch/versions and the active version is
symlinked at ~/.fluxswitch/bin/flux (override with $FLUXSWITCH_BIN).
Make sure the symlink directory is on your PATH:

  export PATH="$HOME/.fluxswitch/bin:$PATH"
`)
}

func listInstalled(home *Home) error {
	installed, err := home.InstalledVersions()
	if err != nil {
		return err
	}
	if len(installed) == 0 {
		fmt.Println("no versions installed yet - run fluxswitch to install one")
		return nil
	}
	current, _ := home.CurrentVersion()
	for _, v := range installed {
		marker := "  "
		if v == current {
			marker = "* "
		}
		fmt.Println(marker + v)
	}
	return nil
}

func switchTo(home *Home, version string) error {
	if !home.IsInstalled(version) {
		fmt.Printf("Downloading flux %s...\n", version)
		if err := Install(home, version); err != nil {
			return err
		}
	}
	if err := home.Activate(version); err != nil {
		return err
	}
	fmt.Printf("Switched flux to %s (%s)\n", version, home.BinLink())
	return nil
}
