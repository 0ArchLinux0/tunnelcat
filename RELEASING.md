# Releasing tailcat

Releases are cut by pushing a version tag. GitHub Actions
(`.github/workflows/release.yml`) then runs
[GoReleaser](https://goreleaser.com/) with the config in
`.goreleaser.yaml`, which builds the artifacts and publishes a GitHub
Release with a changelog generated from the commit log.

## Cutting a release

1. Make sure the Test workflow is green on `main`.
2. Tag and push:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. Watch the Release workflow in the Actions tab. When it finishes,
   the release with all artifacts appears on the
   [Releases page](https://github.com/tailscale/tailcat/releases).

## Artifacts

Each release contains:

* Linux static binaries (tar.gz) for amd64, arm64, and armv7
* Debian (.deb) and RPM (.rpm) packages for the same architectures
* Windows binaries (zip) for amd64 and arm64
* `checksums.txt` with SHA-256 checksums of the above

The binary version is embedded at build time via `-ldflags -X
main.version=...`; `tailcat --version` prints it. Builds made with
`go install github.com/tailscale/tailcat/cmd/tailcat@vX.Y.Z` instead
report the module version from the Go build info.

## Testing locally

To build everything without tagging or publishing, install
[GoReleaser](https://goreleaser.com/install/) and run:

```sh
goreleaser release --snapshot --clean
```

The artifacts land in `dist/` (which is gitignored).
