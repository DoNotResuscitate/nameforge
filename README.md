# Nameforge

Generate TTRPG character and NPC given names, surnames or full names in a local
terminal UI or scriptable CLI. Nameforge uses character-level Markov chains trained on real, licensed name
lists—not AI-authored training data. Select categories, generate a batch, favorite
results, and export text or JSON.

**Works offline on first launch.** The standalone binary includes ten given-name
and eight surname categories and legal notices; no Go, separate data, network, or writable home is
required. Only exports need a writable destination.

## Install

Download your platform archive and `SHA256SUMS` from
[GitHub Releases](https://github.com/DoNotResuscitate/nameforge/releases/latest).
Choose `darwin` for macOS or `linux` for Linux, and `arm64` for Apple Silicon/ARM64
or `amd64` for Intel/AMD x86-64. Native checks cover macOS arm64 and both Linux
architectures; Intel macOS is **cross-built only**.

For example, after downloading a macOS arm64 release, substitute its tag for
`vX.Y.Z`:

```sh
shasum -a 256 nameforge_vX.Y.Z_darwin_arm64.tar.gz
# Compare with the matching filename in SHA256SUMS.
tar -xzf nameforge_vX.Y.Z_darwin_arm64.tar.gz
./nameforge_vX.Y.Z_darwin_arm64/nameforge
```

On Linux use `sha256sum` instead of `shasum -a 256`. See the
[installation guide](docs/RELEASE.md#install-a-standalone-binary) for verification,
PATH installation, corresponding source, and troubleshooting. Archives are
unsigned and not notarized.

## Use

With `nameforge` on your PATH:

```sh
nameforge                       # TUI; requires terminal stdin and stdout
nameforge data list
nameforge generate --category french --category italian --seed 42
nameforge generate --all-categories --mode blend --seed 42 --format json
nameforge generate --name-type surname --category turkish --seed 42
nameforge generate --name-type full --category french --category italian --seed 42
nameforge licenses
```

In the TUI, Space selects a category, Enter generates, `r` regenerates with a fresh
seed, Space in results toggles a favorite, and `e` exports. Press `?` for help or
`l` for legal notices; Ctrl-C quits. See the [TUI walkthrough](docs/TUI.md#walkthrough).

Default `category` mode chooses one selected category equally for each name;
`blend` learns from the union to create hybrid styles. Defaults are 20 unique,
novel names at order 2. Keep the reported seed and options to replay a batch.
Text writes names to stdout and metadata to stderr; JSON includes both.
See [CLI usage](docs/CLI.md) for flags, gender filters, formats, and replay rules.

## Categories and limitations

The embedded corpus contains **10,851 category-specific records**: Dutch, English,
French, German, Italian, Portuguese (Portugal), Spanish, Turkish, and sourced
romanized Greek and Arabic. [Coverage](docs/COVERAGE.md) records counts and gaps;
[source review](docs/ROMANIZED.md) explains the Greek/Arabic scope and exclusions.
An independent Faker-derived surname bundle adds **5,728 records** across the
same eight Latin-script locale categories. Greek/Arabic surnames are unavailable.
Full names pair the same category (or matching blends), given + space + surname;
gender filtering applies only to given names. Select name type in TUI settings
or use `--name-type`; see [composition semantics](docs/CLI.md#surnames-and-full-names).

- Output is Latin-only and stylistically inspired, not guaranteed linguistically
  or historically valid. Greek mixes ancient and modern material; Arabic makes
  no North African regional claim.
- Greek has no source gender labels; use `any`. Unspecified gender is not unisex,
  and all current unisex pools are empty. Missing data never falls back to English.
- Favorites last only for the session; export them before quitting.
- Local packs/preferences and clipboard
  integration are not implemented. `tui --data-dir` is reserved, not a pack loader.

## Development and sources

- [Developer guide](docs/DEVELOPING.md): pinned mise tools, checks, and maintenance.
- [Architecture](docs/ARCHITECTURE.md): boundaries and deterministic generation.
- [Data contract](docs/DATA.md): schema, normalization, provenance, and source locks.
- [Implementation plan](docs/PLAN.md): current milestone status, behavioral contracts,
  and clearly labeled historical acceptance records. M1–M7 are complete.
- [Agent instructions](AGENTS.md): contribution workflow and constraints.

## License

Nameforge is free software under [GNU GPLv3](LICENSE), with no warranty. Eight
given-name and eight surname packs use Faker static arrays (MIT); Greek uses
Wikipedia's supplied Latin/Greek pairs (CC BY-SA 4.0), and Arabic uses Wikidata
statements (CC0). Training data keeps
its separate terms and attribution. Full notices are embedded: run
`nameforge licenses` or press `l` in the TUI. Release archives include notices and
[corresponding source with vendored dependencies](docs/RELEASE.md#archive-contents-and-licenses).
