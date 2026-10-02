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
engine, and M5 headless CLI generation. The corpus contains 10,652 locale-specific
records from Faker v10.6.0. The TUI and release pipeline remain pending. The Go module
path is
`github.com/DoNotResuscitate/nameforge`. Agents should update statuses as work
lands.

### MVP

- Embedded Faker static name arrays, licensed and pinned as defined in DATA.md.
- Searchable multi-select categories, source gender filters and script labels.
- Default per-name category selection and explicit blended Markov models.
- Configurable Markov order, length bounds, count and random seed.
- TUI generation, regenerate, selection, session favorites and text/JSON export.
- Scriptable generation sharing the exact same engine and validation.
- macOS/Linux/Windows binaries, arm64/amd64 where supported by Go.

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

Commands (headless commands available; TUI entry points planned for M6):

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
an existing destination. No clipboard dependency in MVP. Provide resize handling,
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
- Next ready milestone: M6 — Interactive UI.
- Commit: `feat(cli): generate reproducible names from embedded categories`.

### M6 — Interactive UI (pending; depends M5)

- Implement state machine, settings, results, source/seed display, keyboard
  navigation, cancellable asynchronous work, favorites and exports.
- Test update transitions, stale messages, errors, resize and focus without live
  network access; share CLI generation validation rather than duplicate it.
- Acceptance: actual offline first-launch walkthrough, choose French + Italian,
  generate in each mode and export; replay displayed seed via CLI; inspect Greek
  (and Arabic if bundled) script display, error recovery, narrow terminal and
  no-color mode; Ctrl-C restores terminal during active operations.
- Commits: `feat(tui): add corpus and generation controls`,
  `feat(tui): show cancellable generation results`,
  `feat(tui): add session favorites and export`.

### M7 — Verification and binary distribution (pending; depends M6)

- Run `mise run check`, race tests on supported runners, parser/normalizer fuzz
  smoke tests with non-private seeds, CLI integration and manual TUI walkthrough.
- Add mise release-build task and tag-triggered workflow with CGO_ENABLED=0,
  trimpath, version/commit metadata, archives, notices and SHA-256 checksums.
  Target darwin/linux/windows x amd64/arm64; cross-build all, smoke-test natively
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

## 6. Execution and handoff

Critical path: M1 -> M2 -> M3/M4 -> M5 -> M6 -> M7. M3 and M4 may be worked on
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
