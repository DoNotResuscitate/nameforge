# Nameforge

A planned local, Go-based terminal UI for generating TTRPG character/NPC names
with character-level Markov chains trained on real, externally sourced name lists.
Select one or more naming categories, generate a batch, and save favorites.
The initial focus is Mediterranean and Western Europe, plus Turkish, with
accurately sourced North African categories as a possible expansion.

**Status: M1–M4 implemented.** The embedded multilingual corpus contains 10,652
locale-specific records from pinned Faker static arrays. Corpus inspection and
the deterministic Latin-script generation engine are implemented; CLI generation
and the TUI are next (M5/M6). The Go module path is
`github.com/DoNotResuscitate/nameforge`.

## Development handoff

- [Implementation plan](docs/PLAN.md): architecture, behavior, ordered work items,
  acceptance criteria, and suggested atomic Conventional Commits.
- [Data sources](docs/DATA.md): embeddable multilingual datasets and provenance.
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
`data:fetch`, `data:build`, and `data:verify`.
For example, `mise run run -- version` runs the version command. Go
libraries are pinned in `go.mod` / `go.sum`, not installed globally.

## Data and operation

The bundled source is the MIT-licensed static name data from
[Faker](https://github.com/faker-js/faker), extracted at a pinned revision and
embedded with Go's `embed`. Built-in data loads offline without writable storage,
a data download, or a JavaScript runtime.
Agents must never invent training names or sample Faker's generator as a corpus.

The bundle includes French, Spanish, Italian, Portuguese (Portugal), Greek,
Turkish, German, Dutch, English, and broadly labelled Arabic. Use
`mise run run -- data list` and
`mise run run -- data inspect --category french` to inspect coverage.
Greek (55 records) and Arabic (341) retain native scripts and return explicit
unsupported-script errors in the current Latin-only generation engine. Arabic is
not a North African regional pack. Generic buckets have unspecified gender;
none of these lists has a spelling explicitly in both gendered arrays.

### Reproducible corpus maintenance

Ordinary builds and tests use committed assets and never contact name sources.
`data/sources.lock.json` pins Faker v10.6.0, every selected source path and raw
checksum, and the complete MIT notice checksum. `data/quality.json` records
counts, gender/script inventory, rune-length ranges and rejection references;
[the coverage report](docs/COVERAGE.md) records engine smoke results and limits.

```sh
mise run data:verify # offline, no raw cache required
mise run data:fetch  # explicit network operation; checksums, timeouts, retries
mise run data:build  # offline extraction from .local/faker/<revision>/
mise exec -- go run ./cmd/corpus-build verify --rebuild # byte-identical rebuild
```

The strict TypeScript parser accepts literal arrays/objects, quoted strings,
supported escapes, comments and the export wrapper; unknown expressions fail.
Data stays locale-specific to preserve its gender evidence. Normalized duplicate
spellings merge within a locale with all original bucket/index references;
blend-mode training deduplicates across locales. The full upstream notice is
retained in `internal/corpus/assets/builtin/licenses/FAKER-LICENSE`.
Raw caches remain ignored under `.local/`. Source refreshes require explicitly
reviewing the revision, target metadata, checksums, extraction and coverage.
The maintenance-only `pin` command initializes an absent lock at the reviewed
revision; it refuses to overwrite an existing lock.

The TUI will offer searchable category checkboxes, a default mode that chooses
one selected category per generated name, and an explicit blended-model mode for
TTRPG experimentation. Locale labels describe source lists, not promised origins.

[Behind the Name](https://www.behindthename.com/random/) is the category-selection
UX reference. The owner has permission for personal use of its data; optional
personal imports are a later extension. Ancient, mythological, and fictional
categories need independently sourced lists before they can be bundled.
