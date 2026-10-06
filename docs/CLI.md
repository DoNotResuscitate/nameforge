# Headless Nameforge usage

Install a standalone binary using [RELEASE.md](RELEASE.md#install-a-standalone-binary),
or build `./bin/nameforge` using [DEVELOPING.md](DEVELOPING.md).
Generation uses embedded, licensed Faker arrays and sourced romanized Greek/Arabic
lists. It works on first run
without network access, a writable home, a cache, Go, or separately installed data.
Runtime does not download or extract corpora. Local packs are later work;
`--data-dir` is reserved by the TUI and is not accepted by generation.

## Commands

```sh
nameforge --help
nameforge generate --help
nameforge data list
nameforge data inspect --category french
nameforge data list --name-type surname
nameforge data inspect --name-type surname --category turkish
nameforge generate --name-type surname --category turkish --seed 42
nameforge generate --name-type full --category french --category italian --gender feminine --surname-order 1 --seed 42 --format json
nameforge generate --category french --category italian --mode category --seed 42
nameforge generate --category spanish --category turkish --mode blend --seed 42 --format json
nameforge generate --category portuguese-pt --gender feminine --count 10 --order 1 --min-length 3 --max-length 10
nameforge generate --category greek --category arabic --seed 42
nameforge generate --all-categories --mode blend --seed 42 --format json
nameforge licenses
nameforge version
```

`licenses` displays the full application GPLv3 license and warranty/redistribution
summary, pinned dependency notices, and complete Faker MIT notice (including inherited faker.js
notices), Wikimedia attribution, CC BY-SA 4.0 and CC0 1.0 legal texts. Wikipedia-derived
Greek records and adaptations retain CC BY-SA 4.0; Wikidata Arabic records are CC0.
The application's license is GPLv3; see `LICENSE` and [release/source instructions](RELEASE.md).
No-argument headless invocation returns usage mentioning `generate` and exit 2.
With terminal stdin/stdout, no arguments open the interactive category picker.
`nameforge tui` is its explicit entry point; see [TUI usage](TUI.md). Help requires
no TTY.

## Generation options

| Flag | Meaning / default |
| --- | --- |
| `--name-type given\|surname\|full` | Default `given`; separate training data for surnames. |
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
| `--surname-order <n>` | Full-name surname order; default shared `--order`. |
| `--surname-min-length <n>`, `--surname-max-length <n>` | Full-name surname rune bounds; default shared length settings. |
| `--surname-allow-existing[=false]` | Full-name surname novelty override; default shared `--allow-existing`. |

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

### Surnames and full names

Surname coverage is Dutch, English, French, German, Italian, Portuguese
(Portugal), Spanish and Turkish. Greek/Arabic surname data is **unavailable**:
the pinned Faker arrays are native-script, and no Latin source replacement is
bundled. Selecting either returns `empty_selection`, without dropping categories
or substituting English. `--all-categories --name-type surname` selects the eight
surname categories; `--all-categories --name-type full` selects all ten given
categories and fails explicitly on missing surname data.

Full names use one given name, one ASCII space, then one surname. This is an
explicit TTRPG convention, not a culturally complete naming system. Category mode
selects a category uniformly once per output slot and generates **both components
in that category**, retrying there until accepted or bounded exhaustion. Blend
mode trains two separate models on the same selected categories' respective
given/surname unions; each component retains every contributing ID. Independent
cross-category pairing is not implemented.

Gender filtering applies only to given-name training, including the given
component of full names. Surnames use all entries regardless of this setting;
unspecified surname gender remains unspecified, never relabeled unisex.
`--order`, lengths and `--allow-existing` control given names in full mode;
surname settings inherit each shared value unless overridden with `--surname-*`
flags. Overrides require `--name-type full`. Each component has its own effective
1–64-rune bounds, NFC, Latin-only and novelty checks; the combined name can be up
to 129 runes. Components may repeat across a batch; only complete composed
spellings must be unique. The shared `max(1000, count*200)` budget counts
**component sampling attempts**, not pairs; cancellation is checked before each
sample. Partial output/error behavior is unchanged.

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
- `name_type` (also in `options`): `given`, `surname`, or `full`. Older v1
  exports without it describe given names; existing given-name ordered output
  and algorithm are unchanged.
- `seed`, `algorithm_version`, `bundle_hash`: actual unsigned seed, sampling /
  normalization version, and canonical SHA-256 bundle identity.
- `category_ids`, `mode`: sorted effective selection and generation mode.
- `bounds`: effective `{min, max}` rune limits keyed by category ID. Blended
  categories share the union's bounds.
- `options`: normalized request with defaults and the actual seed. Omitted
  `min_length`/`max_length` mean automatic; `allow_existing` defaults to false.
- `names`: ordered objects with `name` and `category_ids` attribution.
- Full-name results also contain `surname_bundle_hash`, `surname_bounds`,
  `component_order: "given-surname"`, `separator: " "`, and normalized
  `options.surname` order/bounds/novelty settings. `bundle_hash` identifies given
  data for full results, surname data for surname-only results. Each full-name
  object has `given` and `surname` objects with exact component `name` spellings
  and `category_ids`. Composition uses the versioned `/full-v1` algorithm.
- `attempts`, `rejections`: attempted samples and counts for `length`, `script`,
  `separators`, `duplicates`, `existing`, `exhausted` rejection classes.
- `complete`: true for a successful CLI batch.

Identical corpus hash(es), algorithm version, options and seed produce identical
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
| 130 | Interrupted generation or interactive session (Ctrl-C / SIGINT). |

Errors are plain text on stderr, including the stable generation error kind when
available. Failure and interruption write no partial generated batch to stdout.
An output device failure can leave already-written bytes and returns exit 1.

Generation attempts are bounded by `max(1000, count*200)`, with an additional raw
per-sample rune cap. `attempts_exhausted` reports how many names were accepted and
suggests a smaller count, broader lengths, lower order or `--allow-existing`.
No fallback names are returned. Existing-name allowance does not create unlimited
diversity, and an unsatisfiable category remains an explicit failure.

## Developer checks

```sh
mise run check
mise run data:verify
mise exec -- go test -race ./...
mise exec -- go test -v ./internal/generator -run TestBuiltinCategoryGenerationSmoke
mise exec -- go test -v ./internal/cli -run TestGenerateBuiltinCategorySmoke
mise exec -- go test -v ./cmd/nameforge -run TestBinaryHeadless
```

The real-binary tests exercise seeded JSON/text replay, help/notices/options and
SIGINT exit 130 outside the checkout with isolated home/PATH. See
[release verification](RELEASE.md#ci-release-workflow-and-target-claims) for native
runners, network-denial scope and testing extracted artifacts with `NAMEFORGE_TEST_BINARY`.
