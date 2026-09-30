# Nameforge

A planned local, Go-based terminal UI for generating TTRPG character/NPC names
with character-level Markov chains trained on real, externally sourced name lists.
Select one or more naming categories, generate a batch, and save favorites.
The initial focus is Mediterranean and Western Europe, plus Turkish, with
accurately sourced North African categories as a possible expansion.

**Status: planning only.** The application and corpus build pipeline are not implemented.
Nameforge is the working project/binary name.

## Development handoff

- [Implementation plan](docs/PLAN.md): architecture, behavior, ordered work items,
  acceptance criteria, and suggested atomic Conventional Commits.
- [Data sources](docs/DATA.md): embeddable multilingual datasets and provenance.
- [Agent instructions](AGENTS.md): workflow and non-negotiable constraints.

## Toolchain

Use [mise](https://mise.jdx.dev/) for tools and exact versions. Go is pinned in
`mise.toml`; after reviewing this repository's configuration:

```sh
mise trust
mise install
mise exec -- go version
```

Build, test, run, format, and verification tasks will be added in milestone M1.
Go libraries will be pinned in `go.mod` / `go.sum`, not installed globally.

## Data and operation

The planned bundled source is the MIT-licensed static name data from
[Faker](https://github.com/faker-js/faker), extracted at a pinned revision and
embedded with Go's `embed`. The binary will work offline on first launch, with
multiple non-English categories and no data download or JavaScript runtime.
Agents must never invent training names or sample Faker's generator as a corpus.

The TUI will offer searchable category checkboxes, a default mode that chooses
one selected category per generated name, and an explicit blended-model mode for
TTRPG experimentation. Locale labels describe source lists, not promised origins.

[Behind the Name](https://www.behindthename.com/random/) is the category-selection
UX reference. The owner has permission for personal use of its data; optional
personal imports are a later extension. Ancient, mythological, and fictional
categories need independently sourced lists before they can be bundled.
