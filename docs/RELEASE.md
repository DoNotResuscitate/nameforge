# Installation, verification and releases

## Install a standalone binary

Download the archive for your system from
[GitHub Releases](https://github.com/DoNotResuscitate/nameforge/releases), together
with `SHA256SUMS`. Choose `darwin` for macOS or `linux` for Linux; `arm64` is Apple
Silicon / ARM64 and `amd64` is Intel/AMD x86-64. Run `uname -m` if unsure.

For a release tag such as `vX.Y.Z`, substitute its actual value below:

```sh
# macOS example (Linux can use sha256sum instead of shasum -a 256)
shasum -a 256 nameforge_vX.Y.Z_darwin_arm64.tar.gz
# Compare with the matching filename in SHA256SUMS.
tar -xzf nameforge_vX.Y.Z_darwin_arm64.tar.gz
./nameforge_vX.Y.Z_darwin_arm64/nameforge version
./nameforge_vX.Y.Z_darwin_arm64/nameforge # interactive with a terminal
./nameforge_vX.Y.Z_darwin_arm64/nameforge generate --all-categories --seed 42
```

With all release files downloaded, `shasum -a 256 -c SHA256SUMS` verifies the whole
set (or `sha256sum -c SHA256SUMS` on Linux). SHA-256 detects corruption; obtain
both the checksum list and archives from the intended repository release.
Optionally place `nameforge` in a directory on your PATH. Retain the notices and
source reference if redistributing it. Archives are not signed or notarized.

The binary alone runs offline on first launch: no Go, Node, downloaded data,
configuration directory, or writable home is required. Only exports need writable
destinations. Every release bundles ten Latin-profile categories with 10,851
records; scope and gender gaps are in [COVERAGE.md](COVERAGE.md), controls in
[TUI.md](TUI.md), and scripted generation in [CLI.md](CLI.md).

## Archive contents and licenses

Each platform archive contains an executable, `BUILD.json` (version, exact commit,
target, Go version, bundle hash and CGO status), this guide, user/source/architecture
documentation, `LICENSE`, `DEPENDENCIES.txt`, `SOURCE.txt`, source locks, coverage,
manifest/category metadata and the full corpus notices under `licenses/`.
Training data is inside the executable. Personal corpora, raw cached pages,
exports and private absolute build paths are excluded.

The application is distributed under **GNU GPLv3** with no warranty. Dependency
copyright/license texts are retained verbatim, including Go's BSD notice and
patent grant. Training data retains separate licenses: Faker MIT, the derived
Greek Wikipedia list CC BY-SA 4.0, and Arabic Wikidata structured data CC0. The
embedded Wikimedia notice preserves source revision URLs, attribution and the
extraction/adaptation description. Redistributing the Greek dataset or its
adaptations requires retaining its attribution and CC BY-SA terms.

`nameforge licenses` displays full application, dependency and corpus legal texts
offline. The TUI displays its copyright/license/warranty summary at startup;
press `l` outside text entry (or from `?` help) to read complete notices with
up/down scrolling. This adds no banner to machine generation output.

Each release also provides `nameforge_<tag>_source.tar.gz` alongside the binaries
and under the same checksum list. It contains the exact application source,
committed corpus/provenance/notices, build/test workflows, and pinned **vendored
dependency source**, including dependencies needed by tests. `SOURCE.txt` points
to that release asset and the exact repository revision. Preserve equivalent
access to corresponding source when redistributing GPL binaries.

## Build from source

In a reviewed checkout, install the pinned developer tools through mise:

```sh
mise trust
mise install
mise run check
mise run data:verify
mise run race
mise run fuzz
mise run release:build -- --version dev
```

`release:build` writes four platform archives, a vendored source archive and
`SHA256SUMS` into ignored `dist/`. It uses the current revision for commit metadata
and archive timestamps. Local dirty-tree builds are useful for verification, but
only clean committed/tagged builds are publishable; `dev`'s download URL is a
placeholder. Existing unrelated files in `dist/` are not packaged or checksummed.
`--out <directory>` supports independent reproducibility checks. Go commands run
through mise with `GOTOOLCHAIN=local`; no implicit toolchain downloads are allowed.

Extract the corresponding source archive and, after installing Go 1.27.1 with
mise, compile entirely from its vendored modules:

```sh
mise trust
GOPROXY=off GOSUMDB=off mise exec -- go build -mod=vendor -trimpath -o nameforge ./cmd/nameforge
./nameforge generate --all-categories --seed 42 --format json
```

This needs no module downloads or upstream corpus cache. The Go toolchain itself
must already be installed. For the same release version output, pass linker flags
`-X github.com/DoNotResuscitate/nameforge/internal/cli.version=<tag>` and
`-X github.com/DoNotResuscitate/nameforge/internal/cli.commit=<commit>`, set
`CGO_ENABLED=0`, and add `-buildvcs=false`. The source archive is sufficient to
build/run the application; making another release archive additionally requires a
Git checkout for revision metadata. Complete test/tooling checks need actionlint
as well as Go. Neither is a runtime dependency.

`notices:build` explicitly derives embedded GPL/dependency texts from pinned
toolchain/module licenses; `notices:verify` compares them byte-for-byte. Ordinary
builds read committed notices. Regenerate/review notices when changing libraries
or Go; `go mod verify` checks the cached modules against their pinned checksums.
Corpus refresh is a separate, explicit maintenance operation described in
[DATA.md](DATA.md).

## CI/release workflow and target claims

Pull requests run the shared distribution workflow with version
`dev`: reproducible four-target archives, vendored-source rebuild, and native tests
of macOS arm64 and both Linux binaries. Intel macOS binaries are cross-built only.
Those CI artifacts are verification builds, not published
releases. **Merging a PR to `main` automatically creates a version tag and publishes
a GitHub Release once its verification succeeds.** Main-branch releases reuse the
same CI/distribution jobs with the real release version, rather than running a
second `dev` build. Manually pushed version tags are also supported.

Automatic versioning starts at **`v0.1.0`** when no stable release tag exists in the
main history. After that, commit messages since the highest stable ancestor tag
determine the next version:

| Changes since the previous stable release | Bump |
| --- | --- |
| `BREAKING CHANGE:` / `BREAKING-CHANGE:` footer, or a Conventional Commit header with `!` | Major |
| `feat:` or `feat(scope):` | Minor |
| All other changes, including fixes, docs, CI and chores | Patch |

The highest applicable bump wins. Standard merges/rebases retain commit messages;
for squash merges, use a Conventional Commit PR title and retain any breaking-change
footer in the squash message. Prerelease/unrelated-branch tags do not advance stable
versions. There is no release-only follow-up PR or manually maintained version file.
`mise run release:version` previews the calculated tag without changing Git.

The release workflow creates the tag at the exact main push revision with its
repository-scoped `GITHUB_TOKEN`, then calls verification and publication directly.
Release runs share a serialized queue, retaining pending merges instead of replacing
them while another release is running.
This avoids relying on a second tag-triggered run: tags created by that token do
not trigger other Actions workflows. No personal access token or additional secret
is required. The tag remains available if verification fails; rerunning that
workflow reuses the same tag. Uploads complete in an automatically published draft,
so an interrupted upload can be retried without a duplicate public release.

The workflow rejects non-main revisions, checks the clean checkout, runs `check`,
`data:verify`, notices/module verification, native race tests and fuzz smoke checks,
and builds all four targets with CGO disabled and trimpath. It compares two full
archive builds and verifies SHA-256. It also rebuilds the bundled source with
`GOPROXY=off GOSUMDB=off` before testing it.

The **natively verified extracted release binaries** run the real CLI and PTY tests on:

| Target | Native GitHub runner | Enforced network denial |
| --- | --- | --- |
| `linux/amd64` | `ubuntu-24.04` | `sudo unshare --net` |
| `linux/arm64` | `ubuntu-24.04-arm` | `sudo unshare --net` |
| `darwin/arm64` | `macos-15` | macOS `sandbox-exec` |

`darwin/amd64` archives are still cross-built and checksummed, but Intel macOS has
no CI/race/native artifact runner. Release notes label it **cross-built only**.

Both headless and interactive tests use an empty home, no developer tools on the
runtime PATH, and no separate data. They verify both modes, Latin-only output,
seed replay, complete licenses, export/favorites/error recovery, compact/no-color
layout, interruptions and terminal restoration. Local Linux tests enforce network
denial only when `NAMEFORGE_TEST_LINUX_SANDBOX=1` is set and passwordless sudo is
available. The Linux wrapper drops to the original runner UID/GID after creating
the namespace, so runtime files and unwritable-home checks keep ordinary user
permissions. macOS tests always enforce network denial. The PTY walkthrough is automated;
maintainers should also follow the [interactive walkthrough](TUI.md#walkthrough)
in their own terminal before a first public release.

For a local native artifact, use an absolute extracted binary path:

```sh
NAMEFORGE_TEST_BINARY="$PWD/dist/nameforge_dev_darwin_arm64/nameforge" mise run release:smoke
```

Publication depends on every native job passing. `VERIFICATION.md` in the release
lists the three native runners and identifies Intel macOS as cross-built only.
All three native jobs must pass; if one is unavailable, publication is blocked.
Local cross-building does **not** imply native verification. Merging to main handles
versioning, tagging and GitHub publication automatically; package-manager publishing
and data refreshes remain separate work. Manual prerelease tags such as
`v0.1.0-rc.1` produce GitHub prereleases.

## Troubleshooting

- **Permission denied / wrong executable format:** extract with `tar` to preserve
  the executable bit, and check OS/CPU selection with `uname -s` / `uname -m`.
- **macOS blocks launch:** these archives are unsigned. Review the source/checksum
  and use the normal macOS Privacy & Security approval process for trusted apps.
- **Headless startup returns usage:** run `generate` explicitly, or open the TUI
  with both stdin/stdout attached to a terminal. `--help` needs no TTY.
- **Empty gender selection:** use `any` for Greek/all categories; unspecified is
  not unisex and current unisex pools are empty. Categories never fall back to English.
- **Insufficient diversity:** lower count/order, broaden lengths, or explicitly
  allow existing names. Preserve the seed/options when diagnosing a batch.
- **Export fails:** the parent directory must exist and be writable; confirm
  overwrite explicitly. Results/favorites remain in memory for retry.
- **Narrow/unstyled terminal:** resize to at least 20x8, or use `tui --no-color` /
  `NO_COLOR`. Ctrl-C restores the terminal. If an external forced kill prevented
  cleanup, your terminal's `reset` command can restore its display.
- **Corrupt built-in assets:** reinstall a verified archive. Runtime never attempts
  a source download to repair an installation.

When reporting a defect, include OS/architecture, `version`, the command or key
sequence, stderr, and replay metadata (bundle/algorithm/seed/options) where useful.
