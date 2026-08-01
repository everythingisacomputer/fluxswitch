# Contributing to fluxswitch

Thanks for your interest! fluxswitch is a small, focused tool — the bar for
contributions is "keeps it simple and doesn't break switching."

## Development setup

You need Go (see `go.mod` for the minimum version). Then:

```sh
git clone https://github.com/everythingisacomputer/fluxswitch
cd fluxswitch
go build -o fluxswitch .
./fluxswitch --help
```

The tool operates entirely on `~/.fluxswitch`, so a dev build is safe to run
on your own machine — worst case, delete that directory to reset.

## Code layout

| File         | Responsibility                                             |
|--------------|------------------------------------------------------------|
| `main.go`    | CLI entry point, flag handling, the switch/install flow    |
| `home.go`    | `~/.fluxswitch` layout, symlink activation, uninstall      |
| `github.go`  | GitHub releases API, HTTP clients, version validation      |
| `install.go` | Download, checksum verification, tarball extraction        |
| `prompt.go`  | The interactive bubbletea picker                           |

## Checks to run before a PR

```sh
go build ./...
go vet ./...
go test ./...
govulncheck ./...   # go install golang.org/x/vuln/cmd/govulncheck@latest
gosec ./...         # go install github.com/securego/gosec/v2/cmd/gosec@latest
```

CI runs build/vet/test and govulncheck on every PR.

The picker itself needs a real terminal, so exercise it manually:
`go build -o fluxswitch . && ./fluxswitch`. A scripted smoke test through a
pseudo-TTY works too:

```sh
{ sleep 1; printf 'q'; } | script -q /dev/null ./fluxswitch
```

## Guidelines

- Keep dependencies minimal; bubbletea is the only direct one.
- Every filesystem write under `~/.fluxswitch` must stay atomic
  (write-to-temp + rename) so a crash never leaves a broken state.
- Version strings are untrusted input: anything that reaches a URL or a
  filesystem path must pass `isValidVersion` first.
- Downloads must remain checksum-verified before extraction.

## Releasing

Releases are fully automated with [GoReleaser](https://goreleaser.com) via
GitHub Actions (`.github/workflows/release.yml`):

```sh
git tag v0.2.0
git push origin v0.2.0
```

That's the whole process. The workflow then:

1. Cross-compiles for darwin/linux × amd64/arm64 (CGO off, version stamped
   via `-X main.version`).
2. Creates a GitHub release with tarballs and `checksums.txt`.
3. Pushes an updated Homebrew formula to
   [everythingisacomputer/homebrew-tap](https://github.com/everythingisacomputer/homebrew-tap),
   which serves both macOS and Linux (Linuxbrew) users.

Notes:

- Versioning is semver: patch for fixes, minor for features (like new picker
  keys), major for breaking changes to the CLI or the `~/.fluxswitch` layout.
- The tap push authenticates with the `TAP_GITHUB_TOKEN` repo secret — a
  GitHub token with write access to `homebrew-tap`. If releases start failing
  at the "homebrew formula" step, that token has expired or lost access.
- To dry-run a release locally without publishing anything:
  `TAP_GITHUB_TOKEN=placeholder goreleaser release --snapshot --clean`
  (artifacts land in `dist/`).
