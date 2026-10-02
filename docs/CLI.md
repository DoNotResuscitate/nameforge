# Headless Nameforge usage

Build with `mise run build`, then run `./bin/nameforge` (or your installed binary).
Generation uses embedded, licensed Faker arrays and sourced romanized Greek/Arabic
lists. It works on first run
without network access, a writable home, a cache, Go, or separately installed data.
Runtime does not download or extract corpora. Local packs and the TUI are later
milestones; `--data-dir` is reserved and is not accepted by generation.

## Commands

```sh
nameforge --help
nameforge generate --help
nameforge data list
nameforge data inspect --category french
nameforge generate --category french --category italian --mode category --seed 42
nameforge generate --category spanish --category turkish --mode blend --seed 42 --format json
nameforge generate --category portuguese-pt --gender feminine --count 10 --order 1 --min-length 3 --max-length 10
nameforge generate --category greek --category arabic --seed 42
nameforge generate --all-categories --mode blend --seed 42 --format json
nameforge licenses
nameforge version
```

`licenses` displays the complete Faker MIT notice (including inherited faker.js
notices), Wikimedia attribution, CC BY-SA 4.0 and CC0 1.0 legal texts. Wikipedia-derived
Greek records and adaptations retain CC BY-SA 4.0; Wikidata Arabic records are CC0.
The application's license is GPLv3; see the repository `LICENSE`.
No-argument headless invocation returns usage mentioning `generate` and exit 2.
The interactive no-argument entry point will arrive with M6. Help requires no TTY.

## Generation options

| Flag | Meaning / default |
| --- | --- |
| `--category <id>` | Repeatable explicit category IDs from `data list`; required unless using `--all-categories`. |
| `--all-categories` | Explicitly select every bundled category; mutually exclusive with `--category`. |
| `--mode category\|blend` | Default `category`; see below. |
| `--gender any\|masculine\|feminine\|unisex` | Default `any`, including unspecified gender. |
| `--count <n>` | 1–1000; default 20 distinct names. |
| `--order <n>` | Character-level Markov order 1–4; default 2. |
| `--min-length <n>` | 1–64 Unicode runes after NFC; default observed minimum. |
| `--max-length <n>` | 1–64 Unicode runes after NFC; default observed maximum, capped at 64. |
| `--seed <n>` | Unsigned 64-bit decimal integer, including 0; omitted seeds use `crypto/rand` and are reported. |
| `--allow-existing` | Permit exact training spellings; still reject duplicates within the batch. |
| `--format text\|json` | Default `text`. |

Both `--flag value` and `--flag=value` are accepted. Boolean flags can be disabled
with `--allow-existing=false` / `--all-categories=false`. All inputs are flags;
positional arguments and unknown flags fail. Explicit zero numeric settings are
invalid (omit length flags for automatic bounds). Minimum must not exceed maximum;
each explicit length flag overrides only that end of the automatic range. Invalid
scalar options fail before corpus loading/training; bounds requiring source data
are checked before training their model.

### Category, gender and script semantics

- IDs are sorted and deduplicated, so selection order does not affect seeded
  output. No category is selected silently and no English fallback exists.
- `category` trains a separate model per category. Each output slot chooses its
  category uniformly, then retries within it. Attribution contains that one ID;
  there is no quota guaranteeing every eligible category appears in a batch.
- `blend` trains one model on the deduplicated union. Larger source lists affect
  more transitions. Each name is attributed to every contributing ID as a hybrid
  style, not to one real culture. Selected script profiles must match.
- Gender filtering uses upstream evidence. `any` includes unspecified entries;
  masculine/feminine include any dual-labeled spellings; `unisex` requires both
  explicit labels. Generic does not mean unisex. The current bundle has no dual
  labels, so unisex filtering fails explicitly. Empty filtered categories are
  never dropped from a multi-category selection.
- Only Latin-script generation is supported. All ten built-in categories have
  Latin profiles, including source-provided romanized Greek and Arabic.
  `--all-categories --seed 42` works in both modes. Future unsupported data still
  returns an explicit `unsupported_script` error; no transliteration occurs.
- Greek mixes ancient/mythological and modern examples; its source provides no
  gender labels, so masculine/feminine/unisex filters return `empty_selection`.
  Accordingly, all-category requests require `--gender any` with this bundle.
  Arabic is broad, not a North African regional corpus. Neither replacement
  claims a single romanization standard or historical frequency; see
  [source review](ROMANIZED.md).
- Names are compared using NFC plus Go's default Unicode lowercase, not
  locale-specific case folding. Turkish `I`, `İ`, `ı`, `i` equivalence can differ
  from orthographic expectations. Output uppercases the first letter at the start
  and after a space/hyphen, preserves other model letters, and does not capitalize
  after an apostrophe. These simple rules do not reconstruct cultural casing.

## Streams and reproducibility

Text stdout contains exactly one name per line, with no headings, metadata or ANSI.
Stderr contains one JSON metadata object on success, with all the JSON fields
below except `names`. Capture them separately when saving a batch:

```sh
nameforge generate --category french --seed 42 > names.txt 2> metadata.json
nameforge generate --category french --seed 42 --format json > batch.json
```

JSON success writes one UTF-8 object followed by a newline to stdout; stderr is
empty. Its v1 fields are:

- `schema_version`: output format version, currently 1.
- `seed`, `algorithm_version`, `bundle_hash`: actual unsigned seed, sampling /
  normalization version, and canonical SHA-256 bundle identity.
- `category_ids`, `mode`: sorted effective selection and generation mode.
- `bounds`: effective `{min, max}` rune limits keyed by category ID. Blended
  categories share the union's bounds.
- `options`: normalized request with defaults and the actual seed. Omitted
  `min_length`/`max_length` mean automatic; `allow_existing` defaults to false.
- `names`: ordered objects with `name` and `category_ids` attribution.
- `attempts`, `rejections`: attempted samples and counts for `length`, `script`,
  `separators`, `duplicates`, `existing`, `exhausted` rejection classes.
- `complete`: true for a successful CLI batch.

Identical corpus hash, algorithm version, options and seed produce identical
ordered output. Replay using the recorded options and `--seed`; automatic bounds
are reproduced from the same bundle. Changing mode, gender, count, bounds, order,
novelty policy or bundle can change output. Category picker order cannot. JSON
consumers must preserve unsigned 64-bit seeds exactly; some floating-point JSON
number representations lose precision for large seeds.

Names are stylistically inspired candidates, not guaranteed linguistically valid
names, origins, or meanings. Generated batches are user output, not training data.

## Failures and exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success, including help/version/licenses. |
| 1 | I/O, corpus, missing filtered data, unsupported script or bounded generation failure. |
| 2 | Invalid command/options, unknown category, incompatible blend or invalid effective bounds. |
| 130 | Interrupted generation (Ctrl-C). |

Errors are plain text on stderr, including the stable generation error kind when
available. Failure and interruption write no partial generated batch to stdout.
An output device failure can leave already-written bytes and returns exit 1.

Generation attempts are bounded by `max(1000, count*200)`, with an additional raw
per-sample rune cap. `attempts_exhausted` reports how many names were accepted and
suggests a smaller count, broader lengths, lower order or `--allow-existing`.
No fallback names are returned. Existing-name allowance does not create unlimited
diversity, and an unsatisfiable category remains an explicit failure.

## Acceptance checks

```sh
mise run check
mise run data:verify
mise exec -- go test -race ./...
mise exec -- go test -v ./internal/generator -run TestBuiltinCategoryGenerationSmoke
mise exec -- go test -v ./internal/cli -run TestGenerateBuiltinCategorySmoke
mise exec -- go test -v ./cmd/nameforge -run TestBinaryHeadless
```

The real-binary test builds through pinned mise Go, runs outside the checkout with
an empty read-only home and no developer tools on PATH, compares seeded JSON/text
in both modes, checks help/notices/options, and verifies Unix SIGINT exits 130 with
empty stdout. On macOS every runtime subprocess also runs under `sandbox-exec`
with network access and filesystem writes denied. Other platforms still exercise
the isolated home/PATH but do not claim an OS-enforced network sandbox.
