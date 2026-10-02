# Nameforge implementation plan

## 1. Goal and delivery boundary

Build a local command-line TUI in Go for TTRPG character/NPC names. Learn character
transitions from real name lists and produce candidates using Markov chains.
Deliver a single executable per target platform with embedded licensed data,
working offline on first launch with no interpreter, hosted service or setup import.
Prioritize Mediterranean and Western European lists plus Turkish; optional North
African packs require specific provenance, not just generic Arabic labels.

The repository includes completed M1 bootstrap, M2 corpus schema and embedded
loading, M3 pinned multilingual data extraction, M4 deterministic generation
engine, M5 headless CLI generation, M5a sourced romanized Greek/Arabic packs, and
M6 interactive UI with session favorites and exports.
The corpus contains 10,851 category-specific records: eight unchanged Faker packs,
486 Wikipedia Greek spellings and 109 Wikidata Arabic spellings. All-category
Latin-only generation works in both modes through the CLI and TUI. M7's verification
and binary-distribution implementation has passed hosted native verification on
all four targets; a human walkthrough and the first public release remain pending. The Go module
path is
`github.com/DoNotResuscitate/nameforge`. Agents should update statuses as work
lands.

### MVP

- Embedded Faker static name arrays plus externally sourced romanized Greek/Arabic
  lists, licensed and pinned as defined in DATA.md. Generated output stays Latin-only.
- Searchable multi-select categories, source gender filters and script labels.
- Default per-name category selection and explicit blended Markov models.
- Configurable Markov order, length bounds, count and random seed.
- TUI generation, regenerate, selection, session favorites and text/JSON export.
- Scriptable generation sharing the exact same engine and validation.
- macOS/Linux binaries, arm64/amd64 where supported by Go.

### Later work

Surname/full-name composition, custom category weights, phonetic models,
syllable constraints, persistent favorites, clipboard integration, model caches,
optional personal Behind the Name imports, historical/mythological packs,
regional North African datasets and package-manager publishing. No LLM or
AI-authored training data. Output is stylistically inspired, not a guarantee of
linguistically valid names or source meanings.

## 2. Tooling and architecture

`mise.toml` pins Go 1.27.1 (resolved with `mise latest go` during planning).
M1 creates the Go module and pins compatible library releases. Use Bubble Tea,
Bubbles and Lip Gloss for the UI; select mutually compatible current module
paths/major versions at M1, committing `go.mod` and `go.sum`. Use stdlib flags,
JSON, HTTP, filesystem and testing; `x/text/unicode/norm` for NFC and `x/term`
for terminal detection. Add dependencies only when a milestone needs them.
Use `GOTOOLCHAIN=local` in mise task environment to avoid implicit Go downloads.

Suggested layout:

```text
cmd/nameforge/main.go        # composition, exit code only
internal/cli/               # commands, flags, streams, terminal detection
internal/corpus/            # schema, validation, normalization, filters
cmd/corpus-build/           # maintenance-only pinned data extraction
internal/corpus/assets/     # committed redistributable data and notices
internal/source/faker/      # static-array extraction; no runtime generator
internal/markov/            # pure training and sampling
internal/generator/         # requests, rejection/uniqueness, result metadata
internal/store/             # local paths and atomic persistence
internal/tui/               # Bubble Tea model/update/view and commands
internal/export/            # text/JSON encoding
docs/                       # design, source notes and user guide
```

Dependencies flow UI/CLI -> generator -> markov/corpus. The maintenance extractor
produces assets consumed through embed.FS. No UI or networking imports in the
Markov engine; no data-network calls in the runtime binary.
Use concrete types and small interfaces at I/O boundaries; avoid a plugin system.
Use the canonical module path from the GitHub remote:
`github.com/DoNotResuscitate/nameforge`.

Define these contracts in M1 before implementing consumers (signatures may use
idiomatic concrete Go types, preserving these semantics):

- `corpus.Record` and `corpus.Manifest`: DATA.md v1 fields.
- `corpus.Select(records, categoryIDs, gender)`: stable ordered category pools;
  `any` includes all, `masculine`/`feminine` include dual-labeled entries,
  `unisex` requires both labels. Upstream generic means unspecified, not unisex.
  Selecting a category with zero matches is an actionable error identifying it;
  never silently drop categories or substitute English.
- `markov.Train(spellings, order)`: immutable model; no filesystem or RNG.
- `generator.Generate(ctx, models, request)`: result plus typed error, using a
  request-local seeded RNG. Result includes actual seed, algorithm version,
  bundle hash, sorted category IDs, mode, per-category effective bounds, options,
  accepted names with category attribution, attempts/rejections and completeness.
- Extractor takes context and injected HTTP/cache boundaries; normal build and
  tests use committed assets with no upstream access. Store serves exports and
  future local packs; built-ins do not require writable filesystem state.

## 3. Markov and generation specification

1. Train a character-level order-N model (`N=1..4`, default 2). Characters are
   Unicode runes, not bytes; all length settings count runes after NFC.
2. For each distinct normalized spelling, prepend N start sentinels and append
   one end sentinel. Sentinels are internal token values, never literal text.
   Accumulate integer next-token transition counts for all context lengths 0..N.
   Each distinct spelling has equal weight; list order is not popularity.
3. Sort training spellings and each transition's successor tokens. Sample by
   cumulative integer weight using a local `math/rand/v2` PCG generator seeded
   with `(seed, seed XOR 0x9e3779b97f4a7c15)`. Never sample by Go map iteration.
   Include RNG/normalization/model version in the algorithm version contract.
4. Start with N start sentinels. Use the longest available suffix context,
   backing off to shorter contexts only if none exists. Sample until END, with a
   hard raw-sampling cap of twice the 64-rune global maximum. NFC-normalize the
   full sample before checking its requested rune length; never truncate it into
   a different name.
5. Validate `count=1..1000`, `1 <= min <= max <= 64`, order range, and unsigned
   64-bit seed. Defaults: count 20, automatic category-derived min/max (DATA.md),
   order 2. Explicit length flags override automatic bounds. In blend mode use
   the selected union's observed bounds. A missing seed is obtained from
   `crypto/rand` and reported in result metadata/TUI.
6. Reject lengths outside bounds, malformed separator placement (leading,
   trailing, repeated separators), duplicate candidates, and, by default,
   exact training spellings. `--allow-existing` permits the latter only.
   Compare names with the same NFC/lowercase key as training. Uniqueness is
   within one batch; regenerated batches may overlap.
7. Cap total attempts at `max(1000, count*200)` and check cancellation between
   attempts. Each attempt has the hard raw-sampling cap in item 4. Exhaustion
   returns a typed error with any partial result and rejection counts; never spin
   forever or fall back to authored/source-picked names. CLI writes no partial
   stdout by default; TUI can show the partial batch with an explicit incomplete
   state.
8. Display casing: uppercase the first letter after the start, a space or hyphen,
   preserving all other model letters. Apostrophes do not trigger capitalization.
   Normalize output to NFC. Document this simple rule rather than claiming it
   reconstructs culturally specific capitalization.
9. The current generator accepts only Latin-script letters, supported combining
   diacritics, and spaces/apostrophes/hyphens. Reject mixed-script samples and
   return an explicit unsupported-script error for categories without a Latin
   profile. Never transliterate or substitute another category.

Reproducibility means identical corpus hash, algorithm version, options and seed
produce identical ordered output across runs/platforms. Pin golden cases and
explicitly version any intentional change to this behavior.

### Multi-category semantics

- Require one or more category IDs, or explicit `--all-categories`. Sort and
  deduplicate IDs before model training and RNG use. Picker order must not alter
  seeded output. Filters are OR across selected categories, AND with gender.
- Default `--mode category`: train a separate model per selected category. For
  each requested output slot choose a category uniformly with the request RNG,
  then retry within that category until an accepted candidate or attempt limit.
  This gives categories equal selection probability rather than favoring larger
  lists or easier-to-generate categories. The global attempt limit still applies.
  Retain category attribution in results. A batch need not include every selected
  category; selections are eligible pools, not a per-category quota.
- `--mode blend`: train one model on the deduplicated union, equal weight per
  distinct spelling. Larger lists influence more transitions; document this.
  This deliberately creates hybrid TTRPG styles. Mark output as blended with
  all contributing category IDs, not as belonging to one real culture.
- Blend only compatible script profiles per DATA.md. Category mode keeps each
  candidate within one category, but the Latin-only output policy still applies.
- Novelty exclusion compares against the union of all selected training names;
  uniqueness applies to the complete batch. Result metadata records mode and
  bundle hash so the same request can be replayed.

## 4. CLI and TUI contract

Commands (headless and interactive entry points available):

```sh
nameforge
nameforge tui --data-dir /path/to/local/data
nameforge data list
nameforge data inspect --category french
nameforge generate --category french --category italian --mode category --seed 42
nameforge generate --category spanish --category turkish --mode blend --count 20
nameforge generate --all-categories --format json --allow-existing
nameforge licenses
nameforge version
```

Generation also accepts `--gender`, `--order`, `--min-length`, `--max-length`.
Reserve `--data-dir` for optional local state/packs; built-ins need no directory.
Default no-argument invocation opens the TUI only with terminal stdin/stdout;
otherwise return usage explaining `generate`, with exit code 2. `--help` works
without a TTY. `generate` requires repeatable `--category` flags or
`--all-categories` (mutually exclusive). TUI opens its bundled category picker
on first launch. Do not default silently to English.

Text generation writes one name per line to stdout, metadata to stderr; JSON
writes one versioned result object including names and reproduction metadata.
Errors go to stderr. Exit codes: 0 success, 1 I/O/data/generation failure,
2 invalid command/options, 130 interrupted. Never emit ANSI in machine output.

TUI state machine: bundled category/filter selection -> generating -> results,
with recoverable error states. Missing/corrupt built-in assets are an installation
error, not an invitation to download a corpus. Generation runs via cancellable
Tea commands, never in `View` or blocking `Update`. Track request IDs so stale
completions cannot replace newer results.

Screen: searchable checkbox category picker with select-all/clear, labels,
scripts, counts and selection summary; settings panel (category/blend mode,
gender, order, count, automatic/explicit lengths, novelty), results list,
active seed, sources and key hints. Controls:
Tab/Shift-Tab focus, arrows or j/k navigation outside text entry, Enter generate,
r regenerate with a fresh seed, Space toggle category in the picker or favorite
in results, e export, ? help,
Esc dismiss/cancel operation, q quit outside text entry, Ctrl-C quit globally.
An explicit entered seed is replayable; show each regenerated batch's new seed.

Export dialog selects current batch or session favorites and text/JSON plus
destination path. Preserve per-batch metadata for favorites from different
generations. Handle write failures without losing results; confirm overwrite of
an existing destination. Clipboard integration is deferred; transitive clipboard
dependencies from UI libraries are acceptable in the MVP. Provide resize handling,
a compact layout, readable no-color mode, and terminal restoration on all exits.

## 5. Ordered implementation milestones

Each suggested commit is a coherent unit, not a requirement to batch the entire
milestone. Include relevant tests with the feature they verify. Milestones remain
pending until their acceptance checks pass.

### M1 — Bootstrap and contracts (complete; no prerequisite milestones)

- Create module, CLI skeleton, version/help command and agreed boundary types.
- Add mise tasks: `fmt`, `fmt-check`, `test`, `vet`, `build`, `run`, `check`.
  `check` runs non-mutating format check, tests, vet and build; build writes bin/.
- Add `.github/workflows/ci.yml` using mise and the same project tasks. CI runs
  without a personal corpus or network calls to name sources.
- Acceptance: clean checkout + mise install builds; help/version run; check
  passes; dependencies pinned; task commands documented.
- Completed: pinned Go and library versions, command routing/help/version,
  corpus and generation boundary types, mise tasks, and CI workflow. The module
  initially used `nameforge`; after the GitHub repository was created it was
  updated to `github.com/DoNotResuscitate/nameforge`. Acceptance commands:
  `mise install`, `mise run check`, `mise run run -- --help`,
  `mise run run -- version`, and `mise exec -- go mod verify` pass.
- Limitation: corpus selection, Markov training, and generation are contracts
  only and remain for M2/M4.
- Next ready milestone: M2 — Corpus schema and embedded loading.
- Commits: `chore(tooling): bootstrap Go module and mise tasks`,
  `feat(cli): add command routing and version output`,
  `ci: verify Go build and tests with mise`.

### M2 — Corpus schema and embedded loading (complete; depends M1)

- Implement DATA.md normalization, schema/version validation, filtering, stable
  serialization/hash, category catalog, embed.FS loading and list/inspect.
- Add traceable redistributable test fixtures and their notice; no invented names.
- Acceptance: reject corrupt/unknown schemas, invalid UTF-8 and empty selections;
  normalization/dedup/hash stable; category/gender semantics tested; generic
  buckets not mistaken for unisex; assets load without writable local storage.
- Completed the v1 bundle validator and read-only loader, NFC name normalization,
  canonical JSONL/category encoding and SHA-256 identities, stable category/gender
  selection, and `data list` / `data inspect`. Tests cover invalid schemas and
  UTF-8, strict fields, content hashes, normalized-spelling duplicates, selection
  ordering, dual-labeled/unset gender behavior, and embedded loading.
- The embedded asset is deliberately only a provenance fixture: three French
  entries from Faker v10.6.0 with their original array buckets/indices, raw source
  checksum, and complete upstream license. It is labeled as a schema fixture and
  is not sufficient training coverage. M3 replaces it with the extracted packs.
- Acceptance checks passed: `mise run check`, `mise run run -- data list`,
  `mise run run -- data inspect --category french`, and `mise exec -- go mod verify`.
- Next ready milestones: M3 — Bundled multilingual dataset, and M4 — Markov engine
  and generation service. Both now depend only on completed M2 and can proceed
  independently.
- Commits: `feat(corpus): validate and filter versioned name corpora`,
  `feat(data): load and inspect embedded category packs`.

### M3 — Bundled multilingual dataset (complete; depends M2)

- Implement pinned static-array extraction and `data:fetch`, `data:build`,
  `data:verify` mise tasks from DATA.md. Commit derived assets, source lock,
  upstream license and category quality report. No generated Faker samples.
- Target the nine core Mediterranean/Western European/Turkish categories in
  DATA.md, plus optional broadly labelled Arabic. Record actual counts and script
  metadata; no English fallback or unsupported North African labels.
- Test string escapes/comments, unsupported TS syntax, malformed records,
  duplicate spellings, generic buckets, missing arrays and checksum failures.
- Acceptance: canonical rebuild identical from locked cache; bundle validates
  offline; source traceability and notices complete; all core target gaps are
  resolved or explicitly reported before marking complete. Once M4 is ready,
  publish generation smoke results for each included Latin-profile category and
  verify that non-Latin categories return the documented unsupported-script
  error (track separately from extraction completion so M3 and M4 can progress
  independently).
- Completed: strict static TypeScript literal extraction; maintenance-only Go
  command with injected HTTP/cache boundaries, pinned URL/checksums, timeouts,
  bounded retries and explicit offline build/verify; `data:fetch`, `data:build`,
  `data:verify` mise tasks; CI offline verification; full upstream notice;
  canonical assets, source lock and machine-readable quality report.
- All nine core targets plus broadly labelled Arabic are included from Faker
  v10.6.0 revision `2cb04231a6ace91a59ebe577c653f4ec66478ca3`. Distinct
  locale-specific counts: Dutch 1,085; English 3,186; French 931; German 1,145;
  Greek 55; Italian 1,700; Portuguese (Portugal) 188; Spanish 227; Turkish 1,794;
  Arabic 341. Total 10,652 accepted of 10,653 source occurrences. Dutch
  `male[571]` is rejected for a control character; provenance/reason retained.
  No within-locale duplicate spellings or explicit dual-gender spellings occur
  in these pinned lists. Generic buckets remain unspecified, never unisex.
- Accepted records retain exact original revision/path/bucket/index references;
  IDs and merging are locale-specific so gender evidence cannot leak between
  categories. Schema, normalization and generator algorithm contracts unchanged.
  Parser tests cover escapes/comments, malformed strings and unknown expressions;
  extraction tests cover normalization/dedup/gender provenance and malformed
  records; maintenance tests cover checksum errors, missing arrays, cancellation,
  bounded retries, notice and quality-report corruption.
- Acceptance checks passed: `mise run check`, `mise run data:fetch`,
  `mise run data:build`, `mise run data:verify`,
  `mise exec -- go run ./cmd/corpus-build verify --rebuild`,
  `mise exec -- go test -race ./...`, `mise exec -- go mod verify`, and CLI
  `data list` / Greek `data inspect`. Fetching into a fresh independent
  `.local/faker-clean` cache and verifying `--rebuild --cache .local/faker-clean`
  produced identical public artifact bytes. Offline verification with
  `--cache .local/absent-cache` passed without a raw cache.
- With M4 complete, per-category fixed-seed smoke checks passed: all eight Latin
  categories produce/replay 20 distinct novel names using seed 42 and defaults;
  Greek and Arabic return explicit `unsupported_script`. French + Italian pass
  picker-order invariance in both modes. The original M4 three-record golden is
  preserved using the exact source-reference subset of the expanded bundle.
  Commands: `mise exec -- go test -v ./internal/generator -run
  TestBuiltinCategoryGenerationSmoke` and `mise exec -- go test
  ./internal/markov ./internal/generator -run '^$' -bench Builtin -benchmem`.
- Coverage, rejection counters, bundle hash and performance measurements are in
  `docs/COVERAGE.md` and `data/quality.json`. Limitations: Greek has only 55 source
  names; Greek/Arabic native-script generation is deferred by policy; broadly
  labelled Arabic has only ten feminine names and no North African regional
  claim. Portuguese/Spanish lists are smaller than other Latin packs. No core
  extraction targets are missing. Smoke success is not a linguistic quality
  guarantee. Unisex selection currently returns explicit empty-selection errors.
- Next ready milestone: M5 — Headless vertical slice.
- Commit: `feat(data): bundle pinned multilingual Faker corpora`.

### M4 — Markov engine and generation service (complete; depends M2)

- Implement section 3 independently of TUI/extractor. Use sourced fixtures and
  clearly non-name token sequences for small transition-count tests.
- Test boundary tokens, weighted transitions, suffix backoff, deterministic
  ordering, Unicode, length validation, source exclusion, duplicate rejection,
  tiny/empty corpus, impossible requests and cancellation.
- Acceptance: fixed-seed golden cases stable; bounded exhaustion returns the
  documented partial result; model supports independent concurrent requests
  without races; benchmark training and 100 model samples with corpus size noted.
  Test category-order invariance, equal category choice, category attribution,
  blended deduplication, script compatibility, automatic bounds, unknown IDs,
  empty filtered categories and generic/unisex distinction.
- Completed the immutable Unicode-rune Markov model, deterministic sorted
  transitions and PCG sampling, category/blend generation, per-category bounds,
  request-local seeded metadata, novelty/uniqueness filters, casing, Latin-only
  output enforcement, and typed bounded/cancellation errors. The review follow-up
  checks rune limits after NFC with a separate 128-rune raw sampling cap. A second
  review follow-up prioritizes supported apostrophe separators before Unicode
  letter classification. Tests cover the listed engine and selection semantics,
  including a fixed-seed `Alix` golden derived from the committed Faker v10.6.0
  French schema fixture and the U+02BC separator.
- Acceptance checks passed: `mise run check`, `mise exec -- go test -race ./...`,
  `mise exec -- go mod verify`, and the training/100-sample benchmark below.
- Baseline on Apple M2 Max (`darwin/arm64`), using the only available corpus at
  this point (three sourced French schema-fixture records): order-2 training
  14,667 ns/op; 100 model samples 22,625 ns/op. These measurements are not
  representative of corpus coverage or name quality. M3 now supplies the
  100-distinct-name end-to-end benchmark and per-category generation smoke checks;
  see `docs/COVERAGE.md`. The original fixture correctly exhausts diversity early.
- Next ready milestone after M3 completion: M5 — Headless vertical slice.
- Commit: `feat(generator): add deterministic Latin-only Markov generation`.

### M5 — Headless vertical slice (complete; depends M3, M4)

- Implement generation flags, stream formats, metadata, exit codes and docs.
- Acceptance: first-run bundled-data -> training -> JSON/text works with an
  empty home and network disabled; repeat seeds across both multi-category modes;
  invalid flags fail before expensive work; insufficient diversity actionable;
  stdout clean; `licenses` displays full embedded notices. Run and record the
  M3 Latin-profile generation smoke checks using default settings and fixed
  seeds; verify explicit unsupported-script errors for non-Latin categories.
- Completed: repeatable category/all-category selection; every planned generation
  flag; decimal uint64 seeds including zero/max; text names with separate JSON
  metadata and a shared versioned JSON export format; clean machine streams;
  full embedded Faker notices via `licenses`; actionable bounded failures with
  no partial stdout; context/SIGINT cancellation and exit 130; documented command,
  option, stream, replay and exit-code semantics in `docs/CLI.md`.
- Exposed `generator.NormalizeRequest` for shared pre-I/O option validation;
  defaults retain their Go API semantics, while frontends reject explicit zero
  numeric settings. Explicit contradictory length bounds now fail before model
  training. Corpus assets, source lock and algorithm version are unchanged.
- Acceptance checks passed on `darwin/arm64`: `mise run check`,
  `mise run data:verify`, `mise exec -- go test -race ./...`,
  `mise exec -- go mod verify`, `mise exec -- go test -v ./cmd/nameforge`, and
  both `mise exec -- go test -v ./internal/generator -run
  TestBuiltinCategoryGenerationSmoke` and `mise exec -- go test -v
  ./internal/cli -run TestGenerateBuiltinCategorySmoke`. All eight Latin categories
  produce 20 distinct novel names at seed 42/defaults; Greek and Arabic return
  `unsupported_script`. M3 smoke counters and the original M4 golden are unchanged.
- The actual executable runs outside the checkout with an empty read-only home,
  no developer tools on PATH, and macOS sandbox denial of network/filesystem writes.
  Text/JSON and ordered seeded replay pass in both French + Italian modes;
  command help/licenses/invalid options and Unix SIGINT are tested at the process
  boundary. CLI tests additionally verify random-seed replay, maximum seed,
  selection-order invariance, source novelty, attribution, I/O failures, gender
  gaps, and suppression of a nonempty partial batch on diversity exhaustion.
- Limitations: interactive startup is deferred to M6; Greek/Arabic and therefore
  `--all-categories` fail under the unchanged Latin-only policy; current unisex
  pools are empty. Other OSes are not natively checked here; the binary acceptance
  test only claims OS-enforced network denial on macOS. No generated batches or
  personal data are committed.
- Next ready milestone: M5a — Romanized Greek/Arabic and all-category generation.
- Commit: `feat(cli): generate reproducible names from embedded categories`.
- Documentation follow-up: simplified Behind the Name references in the README,
  data notes and agent instructions to UX inspiration and a future import adapter.
  Checks: `git diff --check` and a focused documentation content search.

### M5a — Romanized Greek/Arabic and all-category generation (complete; depends M5)

- Required before UI work. Keep generation Latin-only; replace the native-script
  training packs behind `greek` and `arabic` with real, externally sourced
  romanized given-name datasets. Do not generate romanizations, author lists,
  transliterate the Faker arrays, or substitute another category's names.
- Research and select redistributable sources that explicitly provide Greek and
  Arabic names in Latin spelling. Record reviewed category scope, romanization
  conventions, source gender evidence and measured coverage. Keep Arabic broadly
  labeled unless a source establishes a more specific region; no North African
  claim follows from a generic Arabic list. Selected sources and review are recorded below.
- Extend the maintenance-only source lock, extractors and offline verification
  for the selected sources. Pin exact revisions/URLs and raw checksums, preserve
  full license notices and per-record references, and commit reproducibly derived
  assets and quality reports. Keep Faker as the source for the other eight packs.
- Preserve the public `greek`/`arabic` IDs with clear romanized display labels and
  accurate source/script metadata. Train on source-provided Latin spellings with
  the existing NFC/rune and deduplication rules. Use original-script associations
  only as source provenance where provided, never as generated output. Missing
  gender evidence stays unspecified; it cannot become inferred or unisex evidence.
- Ensure every built-in training category has a compatible Latin profile so
  `--all-categories` succeeds in both category and blend modes. Preserve equal
  category choice, attribution, novel-name exclusion, batch uniqueness and the
  existing seeded algorithm; a source refresh changes the bundle hash. Retain
  explicit unsupported-script errors for genuinely unsupported future data.
- Acceptance: each of Greek and Arabic produces/replays 20 distinct novel
  Latin-only names at default settings and seed 42; all ten categories pass the
  same smoke checks. `--all-categories --seed 42` succeeds and replays identically
  in both modes and text/JSON, with complete metadata and correct attribution.
  Non-Latin letters are never emitted. Tests still cover unsupported-script
  rejection using sourced fixtures, gender gaps, Unicode and bounded exhaustion.
- Acceptance: pinned-cache rebuild is byte-identical; `data:verify` works offline;
  provenance/notices are complete; `mise run check` and race tests pass; the actual
  binary completes Greek, Arabic and all-category first-launch generation outside
  the checkout with an empty home, no developer tools and network disabled.
  Update coverage, CLI help/docs and any bundle-dependent expectations; preserve
  the existing sourced French algorithm golden. Do not mark complete until both
  sourced datasets and these checks pass.
- Next ready milestone after completion: M6 — Interactive UI.
- Suggested commits: `feat(data): bundle sourced romanized Greek and Arabic names`,
  `test(cli): verify Latin-only all-category generation`.
- Planning check: `git diff --check` and a focused dependency/script-policy review.
- Completed: Luna source research and pairing review; revision-pinned Wikipedia
  Latin/Greek static lists (CC BY-SA 4.0) and 119 Wikidata entity snapshots (CC0);
  multi-source maintenance lock/fetch/extraction/offline verification; exact
  statement/row provenance and native associations; reviewed rejection accounting;
  complete embedded attribution/legal texts; Latin-profile category labels and
  10-category generation. Other eight packs' records/metadata are unchanged.
- Greek mixes ancient, mythological, Christian and modern material, as requested
  for the early-modern fantasy setting (mid-1600s, forty years after a cataclysm).
  Its 526 source spelling occurrences yield 486 records after 31 rejections and
  nine merges. All gender evidence remains unspecified. Arabic's 137 class/spelling
  occurrences yield 109 records after 25 rejections and three merges: 22 feminine,
  87 masculine, no dual-labeled spellings or regional North African claim.
- Backward-compatible corpus v1 fields add per-file revision/URL and optional
  statement/native/evidence provenance; references match their declared source's
  revision. Original Faker references and sourced French algorithm golden remain
  unchanged. Extractor version is `multisource-static-v1`; sampling/normalization
  algorithm remains `markov-v2/nfc-v1/latin-v1/math-rand-v2-pcg`. Bundle hash:
  `36cd9b6d35049fcd815b233ba12f1565e852157438b5333af7621526a41a4da9`.
- Acceptance checks passed: `mise run check`, `mise run data:fetch`,
  `mise run data:build`, `mise run data:verify`, `mise exec -- go run
  ./cmd/corpus-build verify --rebuild`, `mise exec -- go run ./cmd/corpus-build
  verify --cache .local/absent-cache`, `mise exec -- go test -race ./...`,
  `mise exec -- go mod verify`, individual category smoke tests and
  `TestAllCategoriesLatinNovelReplayAndAttribution`. Rebuild compares all public
  artifacts, notices, quality and the non-embedded native-script fixture byte-for-byte.
- `TestBinaryHeadless` now verifies Greek, Arabic, all-category and French + Italian
  requests in both modes and JSON/text, including ordered replay and complete
  metadata. The actual executable runs outside the checkout with an empty read-only
  home, no developer tools, and macOS sandbox denial of network/filesystem writes.
  Every built-in category produces/replays 20 distinct novel Latin-only names with
  seed 42/defaults. Sourced native-script rejection, gender gaps, Unicode and bounded
  exhaustion remain tested. Coverage, CLI help/docs and source review are updated.
  A final uncached process check, `mise exec -- go test -count=1 -v ./cmd/nameforge`,
  also passed for all eight selection/mode combinations and SIGINT.
- Limitations: no Greek gender labels, empty current unisex pools, heterogeneous
  spelling conventions and community-maintained data with sparse Arabic citations;
  no historical frequency or linguistic-quality guarantee. Native binary checks
  ran on macOS arm64, with OS-enforced network denial only claimed there. No raw
  research pages, synthetic training names or generated batches enter public assets.
- Next ready milestone: M6 — Interactive UI.

### M6 — Interactive UI (complete; depends M5a)

- Implement state machine, settings, results, source/seed display, keyboard
  navigation, cancellable asynchronous work, favorites and exports.
- Test update transitions, stale messages, errors, resize and focus without live
  network access; share CLI generation validation rather than duplicate it.
- Acceptance: actual offline first-launch walkthrough, choose French + Italian,
  generate in each mode and export; replay displayed seed via CLI; generate from
  romanized Greek/Arabic and select all categories in both modes; inspect Latin
  script/source labels, error recovery, narrow terminal and no-color mode;
  Ctrl-C restores terminal during active operations.
- Commits: `feat(tui): add corpus and generation controls`,
  `feat(tui): show cancellable generation results`,
  `feat(tui): add session favorites and export`.
- Completed: terminal-only default/explicit startup and headless help; searchable
  multi-select picker with no implicit selection, all/clear, script/count/gender
  labels and source scope; shared generator validation, every generation setting,
  explicit/fresh seeds; asynchronous cancellable generation and export commands
  with stale-request rejection; result navigation/attribution, incomplete states,
  error recovery; session favorites and atomic JSON/text exports with explicit
  overwrite confirmation; compact resize-aware focused panels, no-color mode,
  scrollable help/source/reproduction details, and terminal restoration.
- Current-batch JSON retains the CLI v1 result format. Favorites JSON uses a
  versioned `batches` collection with each original result and its `selected_names`,
  preserving full per-batch reproduction metadata and completion/counters.
  Built-ins still need no local state; `tui --data-dir` is reserved and does not
  load/persist packs or preferences. Clipboard bindings are disabled. Existing
  corpus assets, provenance, licenses, algorithm and seeded goldens are unchanged.
- Model tests cover focus/text-entry semantics, category selection/filtering,
  validation before work, replay/fresh seeds, supersession/cancellation, missing
  gender recovery, sourced tiny-pool partial exhaustion, favorite toggling,
  cross-batch metadata, export cancellation/write errors/overwrite protection,
  resize, compact confirmation, long-path cursor visibility, help overscroll,
  source scope, no-color views and global quit. Filesystem tests check
  no-clobber publication, cancellation and temporary-file cleanup.
- Acceptance checks passed on `darwin/arm64`: `mise exec -- go test -count=1 -v
  ./cmd/nameforge -run TestBinaryTUI`, `mise run check`,
  `mise exec -- go test -race ./...`, `git diff --check`,
  `mise run data:verify`, and `mise exec -- go mod verify`. After compact-layout
  review fixes, `mise run check`, `mise exec -- go test -race ./internal/tui
  ./internal/store ./internal/cli`, and the uncached `TestBinaryTUI` walkthrough
  passed again. The real-binary
  pseudo-terminal walkthrough starts outside the checkout with an empty read-only
  home, no developer tools on PATH, and macOS sandbox network/home-write denial.
  Keyboard-driven French + Italian, romanized Greek + Arabic, and all-category
  generation pass in both modes with JSON/text exports and exact CLI replay of
  displayed seed 42. It also verifies multi-batch favorites, source/Latin labels,
  gender-error recovery, 40x12 resize/no-color, and Ctrl-C plus external SIGINT
  during active generation, including terminal-mode/alternate-screen restoration.
- Limitations: favorites are session-only; local packs/preferences remain later
  work. Below 20x8 the UI asks for resizing. Native acceptance ran on macOS arm64;
  Ubuntu CI also passed `mise run check` (including the real-binary PTY walkthrough)
  and `mise run data:verify`, without claiming an OS-enforced network sandbox there.
  Release artifacts and their native macOS/Linux checks remain M7. Full corpus
  notices remain available through `licenses`; no generated batches are committed.
- Platform scope clarified after acceptance: macOS and Linux are the supported
  verification/release targets. The documentation follow-up passed `git diff
  --check` and a focused platform-target review.
- Export follow-up: switching format updates the filename extension to `.txt`
  or `.json`, preserving the directory/basename and adding a suffix when absent.
  Regression checks cover default/custom paths, dotfiles, empty input and switching
  back to JSON; the binary walkthrough checks the automatic text filename. Checks:
  `mise run check`, `mise exec -- go test -count=1 -v ./cmd/nameforge -run
  'TestBinaryTUI/category/french'`, and `git diff --check` passed.
- Clipboard review disposition: the project owner accepted Bubbles text input's
  transitive clipboard dependency and relaxed the former dependency prohibition.
  Clipboard paste remains disabled. Documentation checks: `git diff --check` and
  a focused clipboard-contract review passed.
- Next ready milestone: M7 — Verification and binary distribution.
- Commit: `feat(tui): add offline interactive generation and session exports`.

### M7 — Verification and binary distribution (implemented; acceptance pending; depends M6)

- Run `mise run check`, race tests on supported runners, parser/normalizer fuzz
  smoke tests with non-private seeds, CLI integration and manual TUI walkthrough.
- Add mise release-build task and tag-triggered workflow with CGO_ENABLED=0,
  trimpath, version/commit metadata, archives, notices and SHA-256 checksums.
  Target darwin/linux x amd64/arm64; cross-build all, smoke-test natively
  on available OS runners, and clearly identify targets only cross-built.
- Verify artifacts contain intended built-in packs/notices but no personal corpus,
  cached pages or private paths; run a binary outside the checkout with an empty
  home, network disabled, no Go and no separately installed data.
- Document category coverage/modes, offline use, reproducibility, controls,
  troubleshooting, architecture, data sources and dependency notices. The
  GitHub repository's `LICENSE` is GNU GPLv3; include it and verify release
  artifacts and interactive legal notices comply before public release.
- Acceptance: clean-checkout workflow and data:verify pass, checksummed binaries
  build, offline first-launch generation works, no runtime dependency beyond binary.
- Commits: `ci(release): build checksummed cross-platform binaries`,
  `docs: document bundled categories and terminal usage`.
- Implemented: pinned actionlint 1.7.12 and workflow validation in `check`;
  native macOS/Linux amd64/arm64 CI and race tasks; public non-name token parser
  and NFC normalizer fuzz targets; extracted-binary CLI/PTY acceptance support;
  opt-in Linux empty-network-namespace enforcement alongside macOS sandboxing.
- Added `release:build` for all four CGO-disabled, trimpath targets with version,
  exact revision and corpus metadata; deterministic sorted tar/gzip archives,
  revision-derived timestamps, explicit public-file boundaries, private-path/build
  audits and SHA-256 checksums. Builds retain complete application/dependency/data
  notices and documentation. The corresponding-source archive includes committed
  corpus/provenance and pinned vendored dependencies for download-free module builds.
- Pull-request/main CI and tag-triggered releases share distribution jobs that
  compare two independent archive builds and test every extracted target natively.
  Tag-triggered release CI reuses clean-checkout/native verification, compares two
  independent archive builds, rebuilds vendored source with module downloads off,
  smoke-tests every extracted target natively, and publishes only after all pass.
  Release verification notes identify each actual runner and network sandbox;
  unavailable runners block publication rather than receiving a native-check claim.
- Full GNU GPLv3 and pinned Go/runtime/test dependency legal texts are derived into
  an explicit embedded notice directory. `licenses` exposes these plus unchanged
  corpus notices offline; the TUI shows copyright/license/no-warranty information
  and an `l`-activated scrollable full-notice screen. Tests check exact embedded
  GPL bytes, offline notice availability, text-entry semantics, legal/help switching
  and export-help recovery. Added installation, source/release, architecture and
  troubleshooting guides. Corpus assets, source locks, bundle hash, generation
  algorithm and seeded outputs are unchanged.
- Local checks passed on `darwin/arm64`: `mise install`, `mise run check`,
  `mise run race`, `mise run fuzz` (10-second parser and normalizer smoke runs),
  `mise run data:verify`, `mise run notices:verify`, `mise exec -- go mod verify`,
  and `git diff --check`. `mise run release:build -- --version dev` and a second
  build with `--out dist/rebuild` produced byte-identical SHA256SUMS for all four
  binary archives and the vendored source archive; `shasum -a 256 -c SHA256SUMS`
  passed. Native extracted macOS arm64 `release:smoke` passed with empty read-only
  home, no tools/data and OS-enforced network denial, including legal screens,
  both modes, exports, replay and terminal restoration. A vendored source rebuild
  with `GOPROXY=off GOSUMDB=off` passed the same binary acceptance suite.
- Native Linux arm64 acceptance also passed in Ubuntu 24.04 under the local
  ARM64 Linux container engine: the cross-compiled Go test executable runs the
  extracted release binary with no Go or data installation. Both Docker network
  denial/read-only filesystem and the CI `sudo unshare --net` wrapper passed full
  CLI/PTY tests, including active Ctrl-C/SIGINT forwarding and terminal restoration.
  The Ubuntu image digest was
  `sha256:a853f94d226358a79c740cfc7bce0c289748f3fe3488d921d038ccd752c61b60`.
  This is native Linux arm64 container acceptance, not a hosted-runner/race claim.
- First hosted run: both macOS check/race jobs passed. Linux exposed root-owned
  exports from the test network wrapper. The wrapper now drops back to the test
  runner's UID/GID after creating the network namespace, preserving normal home
  and export permissions. The full compiled CLI/PTY suite passed as a non-root
  Ubuntu container user using this corrected wrapper.
- Hosted acceptance passed in [CI run 36970584654](https://github.com/DoNotResuscitate/nameforge/actions/runs/36970584654)
  for [PR #12](https://github.com/DoNotResuscitate/nameforge/pull/12), revision
  `d99a531`: all four clean-checkout native `check`/`data:verify`/notice/module/race
  jobs, Linux amd64 fuzz smoke, reproducible four-target/source distribution build,
  vendored-source download-free rebuild plus acceptance, and all four extracted
  native artifact CLI/PTY smoke jobs passed. There are no cross-build-only targets
  in this verified CI artifact set. Linux runtimes use the corrected non-root
  empty-network-namespace wrapper; macOS runtimes use sandbox-exec.
- Remaining acceptance: perform the human terminal walkthrough and exercise the
  tag-triggered publication path for the first release. Local artifacts are
  dirty-tree `dev` builds; CI artifacts are clean-checkout `dev` verification
  builds, not public releases. M7 remains acceptance-pending until its manual and
  first-publication checks actually pass; no later mandatory milestone is ready
  before M7 closeout. No corpus, source lock or seeded algorithm changes occurred.
- CI follow-up: preserve the ruleset's required `check` context with a stable
  aggregate job that fails if any native/distribution job fails or is skipped.
  The native matrix otherwise changes status names and leaves `check` expected
  forever. Checks: `mise run workflow:check` (also in `mise run check`) and
  `git diff --check` passed. The aggregate uses `always()` and explicitly requires
  success from both dependencies, so failure/skipping cannot satisfy branch rules.
- Codex review follow-up: narrow/short headers retain copyright, GPLv3 and NO
  WARRANTY down to the supported 20x8 minimum, rather than truncating the legal
  terms or removing them in compact mode. Header-aware help paging and resize
  regression checks preserve legal visibility and the compact picker row. Checks:
  `mise run check`, `mise exec -- go test -race ./internal/tui` and `git diff
  --check` passed, including the real-binary CLI/PTY walkthrough.
- Owner-requested release automation: merging to main now calculates a stable
  version, creates its tag and directly invokes verification/publication in the
  same workflow, rather than requiring a manual tag. Initial version is v0.1.0;
  breaking changes bump major, `feat` bumps minor and every other main update
  bumps patch, including docs/CI/chore-only merges. Main release runs replace
  duplicate main `dev` CI; PR verification and the stable required `check` remain.
  Existing tags at the same revision are reused on retries, and publication uploads
  through an automatically published draft. A serialized `queue: max` retains
  pending merge runs while an earlier release runs. GITHUB_TOKEN-created tags do not
  trigger another run, so no PAT, release PR or separate version file is needed.
  `release:version` is a read-only preview; tests cover conventional bump priority,
  numeric tag ordering, bootstrap, prerelease/branch exclusion and annotated-tag
  retry behavior. Checks passed: `mise run check`, `mise exec -- go test -race
  ./cmd/release-version`, `mise run release:version` (v0.1.0 for this untagged
  repository), and `git diff --check`. First merge-driven publication is still
  pending the merge. actionlint 1.7.12 predates GitHub's documented concurrency
  `queue` property; workflow validation suppresses only that stale-schema diagnostic.

## 6. Execution and handoff

Critical path: M1 -> M2 -> M3/M4 -> M5 -> M5a -> M6 -> M7. M3 and M4 may be worked on
independently after M2 contracts settle; coordinate go.mod changes. This is a
dependency map for future implementers, not an instruction to spawn agents.

Repository handoff: `origin` is `git@github.com:DoNotResuscitate/nameforge.git`.
The remote's original LICENSE-only initial commit has been merged into local
history as `chore(repo): merge remote license history`; its GNU GPLv3 `LICENSE`
is now present in the working tree.

For each handoff record: milestone status, changed contracts, commands/checks and
their results, extracted counts and source revisions where relevant, unresolved
limitations, and next ready work. Never claim measured corpus quality from mocks.

Key engineering risks: upstream data shape changes (pinned extraction), small
corpora (bounded rejection and visible counts), uneven category coverage (reports),
Unicode/casing and native-script usability (NFC/rune/rendering tests),
non-deterministic iteration (sorted transitions/goldens), terminal responsiveness
(async commands/cancellation), and dataset updates (hash/versioned provenance).
