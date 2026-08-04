# fluxswitch

[![ci](https://github.com/everythingisacomputer/fluxswitch/actions/workflows/ci.yml/badge.svg)](https://github.com/everythingisacomputer/fluxswitch/actions/workflows/ci.yml)
[![release](https://github.com/everythingisacomputer/fluxswitch/actions/workflows/release.yml/badge.svg)](https://github.com/everythingisacomputer/fluxswitch/actions/workflows/release.yml)
[![latest release](https://img.shields.io/github/v/release/everythingisacomputer/fluxswitch)](https://github.com/everythingisacomputer/fluxswitch/releases/latest)
[![buy me a coffee](https://img.shields.io/badge/buy%20me%20a%20coffee-☕-ffdd00?logo=buymeacoffee&logoColor=black)](https://buymeacoffee.com/everythingisacomputer)

Switch between versions of the [Flux CLI](https://github.com/fluxcd/flux2) the
same way [tfswitch](https://github.com/warrensbox/terraform-switcher) does for
Terraform.

![fluxswitch demo: the interactive picker installs and switches to a version, then flux --version confirms it](demo.gif)

## Install

With Homebrew (macOS or Linux):

```sh
brew tap everythingisacomputer/tap
brew trust everythingisacomputer/tap   # newer Homebrew requires trusting third-party taps
brew install fluxswitch
```

Or with Go:

```sh
go install github.com/everythingisacomputer/fluxswitch@latest
```

Then put the fluxswitch bin directory on your PATH (add to your shell profile):

```sh
export PATH="$HOME/.fluxswitch/bin:$PATH"
```

## Usage

```sh
fluxswitch            # interactive picker (arrows, / search, d download, u uninstall)
fluxswitch 2.3.0      # switch to a specific version, downloading if needed
fluxswitch --latest   # switch to the latest Flux release
fluxswitch --list     # list installed versions (* marks active)
```

## How it works

Versions are downloaded from the official fluxcd/flux2 GitHub releases and
stored under `~/.fluxswitch/versions/<version>/flux`. Switching atomically
repoints the `~/.fluxswitch/bin/flux` symlink at the chosen version, so `flux`
always resolves to the active one.

Set `FLUXSWITCH_BIN` to change where the symlink lives (e.g.
`FLUXSWITCH_BIN=/usr/local/bin/flux`).

## Support

If fluxswitch saves you time, you can
[buy me a coffee](https://buymeacoffee.com/everythingisacomputer). ☕
