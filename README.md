# Nameforge

A local, Go-based terminal UI for generating TTRPG character/NPC names
with character-level Markov chains trained on real, externally sourced name lists.
Select one or more naming categories, generate a batch, and save favorites.
The initial focus is Mediterranean and Western Europe, plus Turkish, with
accurately sourced North African categories as a possible expansion.

**Status: M1–M7 complete; [v0.1.0 released](https://github.com/DoNotResuscitate/nameforge/releases/tag/v0.1.0).** The embedded multilingual corpus contains 10,851
category-specific records from pinned Faker arrays, Wikipedia and Wikidata.
Corpus inspection and reproducible Latin-only generation work offline for all
ten categories, including romanized Greek and Arabic. The interactive picker,
settings, session favorites, and text/JSON exports are available. M7 adds
checksummed four-target release archives, full legal notices, three-target native verification
and a merge-driven automatic tag/release pipeline. The owner confirmed human
all-feature testing of v0.1.0 on macOS 15 arm64 and CachyOS amd64.
The Go module path is
`github.com/DoNotResuscitate/nameforge`.

Supported platforms are macOS and Linux, arm64 and amd64. See
[installation and releases](docs/RELEASE.md) for standalone binaries, checksums,
corresponding source, offline builds and troubleshooting.
Native checks cover macOS arm64 and Linux arm64/amd64; Intel macOS binaries are
cross-built only.

## Development handoff

- [Implementation plan](docs/PLAN.md): architecture, behavior, ordered work items,
  acceptance criteria, and suggested atomic Conventional Commits.
- [Data sources](docs/DATA.md): embeddable multilingual datasets and provenance.
- [Architecture](docs/ARCHITECTURE.md): boundaries and deterministic generation.
- [Agent instructions](AGENTS.md): workflow and non-negotiable constraints.

## Toolchain

Use [mise](https://mise.jdx.dev/) for tools and exact versions. Go and the
project commands are pinned in `mise.toml`; after reviewing this repository's
configuration:

```sh
mise trust
mise install
mise run check
```

Available tasks are `fmt`, `fmt-check`, `test`, `vet`, `build`, `run`, `check`,
`data:fetch`, `data:build`, `data:verify`, `race`, `fuzz`, `workflow:check`,
`notices:build`, `notices:verify`, `release:version`, `release:build`, and `release:smoke`.
For example, `mise run run -- version` runs the version command. Go
libraries are pinned in `go.mod` / `go.sum`, not installed globally.

## Generate names

```sh
mise run build
./bin/nameforge # opens the TUI with terminal stdin/stdout
./bin/nameforge data list
./bin/nameforge generate --category french --category italian --seed 42
./bin/nameforge generate --category spanish --category turkish --mode blend --seed 42 --format json
./bin/nameforge generate --category greek --category arabic --seed 42
./bin/nameforge generate --all-categories --mode blend --seed 42 --format json
./bin/nameforge licenses
```

Select categories explicitly. The default `category` mode chooses a selected
category uniformly for each name; `blend` learns from the deduplicated union and
produces hybrid TTRPG styles. Defaults are 20 distinct novel names, Markov order 2,
and observed category-derived length bounds. Text writes one name per line to
stdout and reproduction metadata to stderr. JSON writes one versioned result
containing names, category attribution, seed, corpus hash, options and counters.
Keep the reported seed to replay a batch; omitted seeds are random.

See [CLI usage](docs/CLI.md) for all flags, formats, replay rules and exit codes.
See [TUI usage](docs/TUI.md) for keyboard controls, favorites, exports and no-color mode.
The binary requires no separate data, writable home, Go installation or network.

## Data and operation

Eight packs use MIT-licensed static name data from
[Faker](https://github.com/faker-js/faker). Greek uses Wikipedia's supplied
Latin/Greek pairs (CC BY-SA 4.0); Arabic uses Wikidata's supplied Latin name
statements (CC0). Every source is revision-pinned, checksum-locked and embedded
with Go's `embed`. Built-in data loads offline without writable storage,
a data download, or a JavaScript runtime.
Agents must never invent training names or sample Faker's generator as a corpus.

The bundle includes French, Spanish, Italian, Portuguese (Portugal), Greek,
Turkish, German, Dutch, English, and broadly labelled Arabic. Use
`mise run run -- data list` and
`mise run run -- data inspect --category french` to inspect coverage.
Greek (486 records) mixes ancient, mythological, Christian and modern examples,
suited to the intended early-modern fantasy setting rather than a claim of
historically exact 1600s usage. Its source does not label gender: use `--gender any`.
Arabic (109 records) remains broadly labeled, with no North African regional
claim. Generic buckets mean unspecified; all current unisex pools are empty.
`--all-categories` works in both modes with the default gender filter.
See [romanized source review](docs/ROMANIZED.md) for scope, spelling conventions,
exclusions, licensing and provenance.

### Reproducible corpus maintenance

Ordinary builds and tests use committed assets and never contact name sources.
`data/sources.lock.json` pins the supplemental source revisions, exact URLs and
raw checksums, full license texts and the original `data/faker.lock.json` checksum.
The latter retains Faker v10.6.0 and its complete MIT notice. `data/quality.json` records
counts, gender/script inventory, rune-length ranges and rejection references;
[the coverage report](docs/COVERAGE.md) records engine smoke results and limits.

```sh
mise run data:verify # offline, no raw cache required
mise run data:fetch  # explicit network operation; checksums, timeouts, retries
mise run data:build  # offline extraction from the ignored .local/faker/ cache
mise exec -- go run ./cmd/corpus-build verify --rebuild # byte-identical rebuild
```

The strict TypeScript parser accepts literal arrays/objects, quoted strings,
supported escapes, comments and the export wrapper; unknown expressions fail.
The Greek extractor reads only reviewed static wikitext lists and literal links;
the Arabic extractor validates entity revisions, language and given-name claims.
Data stays locale-specific to preserve its gender evidence. Normalized duplicate
spellings merge within a locale with all original bucket/index references;
blend-mode training deduplicates across locales. Full source notices and attribution
are retained in `internal/corpus/assets/builtin/licenses/` and shown by `licenses`.
Raw caches remain ignored under `.local/`. Source refreshes require explicitly
reviewing the revision, target metadata, checksums, extraction and coverage.
The maintenance-only `pin --roster <research.json>` command initializes an absent
supplemental lock from reviewed QID/revision metadata; it takes spellings only from
upstream source files and refuses to overwrite an existing lock.

The TUI offers searchable category checkboxes, a default mode that chooses
one selected category per generated name, and an explicit blended-model mode for
TTRPG experimentation. Locale labels describe source lists, not promised origins.

[Behind the Name](https://www.behindthename.com/random/) is the category-selection
UX reference. Ancient, mythological, and fictional categories need independently
sourced lists before they can be bundled.

## License

Nameforge is free software under [GNU GPLv3](LICENSE), with no warranty.
Full application, dependency and corpus notices are embedded: run
`nameforge licenses` or press `l` in the TUI. Redistributable training data retains
its MIT / CC BY-SA 4.0 / CC0 terms. Release archives preserve these notices and
provide corresponding source with vendored dependencies; see [RELEASE.md](docs/RELEASE.md).
