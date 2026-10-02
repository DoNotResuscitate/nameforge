# Developing Nameforge

For standalone installation and normal use, start with the [README](../README.md),
[CLI guide](CLI.md), or [TUI guide](TUI.md). This guide covers work in a reviewed
Git checkout. Follow [AGENTS.md](../AGENTS.md) for branching, commits, and handoff.

## Pinned tools and checks

Use [mise](https://mise.jdx.dev/) for all developer tool versions and Go commands.
`mise.toml` pins Go 1.27.1 and actionlint 1.7.12; `GOTOOLCHAIN=local` prevents
implicit Go toolchain downloads. Dependencies are pinned in `go.mod`/`go.sum`.

```sh
mise trust
mise install
mise run check
mise run data:verify
mise exec -- go mod verify
mise run build
./bin/nameforge --help
```

`check` runs non-mutating formatting, tests, vet, workflow validation, notice
verification, and build. Dependencies/toolchains may need downloads initially;
ordinary builds, tests, and corpus verification never contact name sources.

| Task | Purpose |
| --- | --- |
| `fmt` / `fmt-check` | Format Go / check formatting without changes |
| `test` / `vet` / `build` | Tests / static checks / binary in `bin/` |
| `run` | Run CLI, e.g. `mise run run -- version` |
| `race` / `fuzz` | Native race tests / public-token parser and normalizer fuzz smoke |
| `workflow:check` | Validate GitHub workflows |
| `data:fetch` / `data:build` / `data:verify` | Explicit source fetch / cached rebuild / offline verification |
| `notices:build` / `notices:verify` | Derive full legal texts / check committed bytes |
| `release:version` / `release:build` / `release:smoke` | Preview version / package archives / test an extracted native binary |

Run `mise run race` and `mise run fuzz` for broader verification. Real-binary
CLI/PTY tests are part of `test`; native runner/network-sandbox claims and artifact
testing are documented in [RELEASE.md](RELEASE.md#ci-release-workflow-and-target-claims).

## Reproducible corpus maintenance

Training data must be externally sourced static arrays/lists, never authored
names, generated Faker output, or project-generated romanizations. Preserve
exact source references, accurate category/gender scope, pinned checksums, and
complete notices. [DATA.md](DATA.md) is the schema/provenance contract;
[ROMANIZED.md](ROMANIZED.md) records reviewed Greek/Arabic selection.

```sh
mise run data:verify # offline; no raw cache required
mise run data:fetch  # explicit network operation, checksum-locked sources
mise run data:build  # offline extraction from .local/faker/
mise exec -- go run ./cmd/corpus-build verify --rebuild
```

`data/sources.lock.json` pins supplemental URLs/revisions/checksums and the original
`data/faker.lock.json` checksum. `data/quality.json` retains source accounting;
[COVERAGE.md](COVERAGE.md) records measured coverage and smoke results.

The strict TypeScript parser rejects unknown expressions. Greek extraction uses
reviewed static wikitext lists; Arabic extraction validates revision, language,
and given-name claims. Deduplication retains source occurrences and gender
evidence within each category. Raw caches, research pages, personal corpora,
generated batches, and correspondence belong in ignored local storage, not commits.

Refreshes are explicit reviewed changes, never side effects of build/release.
The maintenance-only `pin --roster <research.json>` command initializes an absent
supplemental lock from reviewed QIDs/revisions and refuses to overwrite an existing
lock. Spellings come only from upstream snapshots. Rebuild byte-for-byte and
review coverage/notices before committing changed public assets.

## Legal notices and distribution

`notices:build` derives full pinned toolchain/module notices and the project GPL
text; `notices:verify` compares them byte-for-byte. Review/regenerate notices when
changing Go or dependencies. Corpus licenses remain separately attributed.

See [RELEASE.md](RELEASE.md#build-from-source) for vendored offline builds, release
archives, checksums, automatic versioning, and the clean-tree publication boundary.
Built-in assets and seeded algorithm behavior must not change incidentally.

## Status and future work

M1–M7 are complete. [PLAN.md](PLAN.md#current-status) is the current delivery
summary; its historical records retain the checks and limitations at each stage.
Surname generation (#10), full-name composition (#11), persistent preferences,
regional packs, and other expansion work are not part of the delivered MVP.
