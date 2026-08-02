# fluxswitch

Switch between versions of the [Flux CLI](https://github.com/fluxcd/flux2) the
same way [tfswitch](https://github.com/warrensbox/terraform-switcher) does for
Terraform.

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
