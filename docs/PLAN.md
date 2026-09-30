# Nameforge implementation plan

## 1. Goal and delivery boundary

Build a local command-line TUI in Go that learns character transitions from real
name lists and produces new candidate names using Markov chains. Deliver a single
executable per target platform, with no runtime interpreter or hosted service.
The corpus is imported locally once; generation is offline thereafter.

The repository currently contains planning documents and a mise Go pin only.
No application code, downloaded corpus, remote, or release pipeline exists yet.
All milestones below are pending. Agents should update statuses as work lands.

### MVP

- Personal Behind the Name import by usage/category, including pagination.
- Local corpus selection and source gender filters.
- Configurable Markov order, length bounds, count and random seed.
- TUI generation, regenerate, selection, session favorites and text/JSON export.
- Scriptable generation sharing the exact same engine and validation.
- macOS/Linux/Windows binaries, arm64/amd64 where supported by Go.

### Later work

Surname/full-name composition, mixed weighted cultures, phonetic models,
syllable constraints, persistent favorites, clipboard integration, model caches,
embedded redistributable datasets, and package-manager publishing. No LLM or
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
internal/source/btn/        # acquisition, cache, HTML extraction
internal/markov/            # pure training and sampling
internal/generator/         # requests, rejection/uniqueness, result metadata
internal/store/             # local paths and atomic persistence
internal/tui/               # Bubble Tea model/update/view and commands
internal/export/            # text/JSON encoding
docs/                       # design, source notes and user guide
```

Dependencies flow UI/CLI -> generator -> markov/corpus. Source importer and store
are composed by CLI commands. No UI or networking imports in the Markov engine.
Use concrete types and small interfaces at I/O boundaries; avoid a plugin system.
Choose the final Go module path from the actual remote if one exists at M1;
otherwise use `nameforge` locally and record the later rename prerequisite.

Define these contracts in M1 before implementing consumers (signatures may use
idiomatic concrete Go types, preserving these semantics):

- `corpus.Record` and `corpus.Manifest`: DATA.md v1 fields.
- `corpus.Select(records, usage, gender)`: stable ordered training spellings;
  `any` includes all, `masculine`/`feminine` include dual-labeled entries,
  `unisex` requires both labels. Empty selection is an actionable error.
- `markov.Train(spellings, order)`: immutable model; no filesystem or RNG.
- `generator.Generate(ctx, model, request)`: result plus typed error, using a
  request-local seeded RNG. Result includes actual seed, algorithm version,
  corpus hash, options, accepted names, attempt/rejection totals and completeness.
- Importer takes context and injected HTTP/cache boundaries; store publishes only
  validated complete updates. Test without contacting the live site.

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
   backing off to shorter contexts only if none exists. Sample until END.
   At maximum length, accept only if the next sampled token is END; otherwise
   reject the candidate rather than truncate it into a different name.
5. Validate `count=1..1000`, `1 <= min <= max <= 64`, order range, and unsigned
   64-bit seed. Defaults: count 20, min 3, max 12, order 2. A missing seed is
   obtained from `crypto/rand` and reported in result metadata/TUI.
6. Reject lengths outside bounds, malformed separator placement (leading,
   trailing, repeated separators), duplicate candidates, and, by default,
   exact training spellings. `--allow-existing` permits the latter only.
   Compare names with the same NFC/lowercase key as training. Uniqueness is
   within one batch; regenerated batches may overlap.
7. Cap total attempts at `max(1000, count*200)` and check cancellation between
   attempts. Each attempt is also bounded by max length. Exhaustion returns a
   typed error with any partial result and rejection counts; never spin forever
   or fall back to authored/source-picked names. CLI writes no partial stdout
   by default; TUI can show the partial batch with an explicit incomplete state.
8. Display casing: uppercase the first letter after the start, a space or hyphen,
   preserving all other model letters. Apostrophes do not trigger capitalization.
   Normalize output to NFC. Document this simple rule rather than claiming it
   reconstructs culturally specific capitalization.

Reproducibility means identical corpus hash, algorithm version, options and seed
produce identical ordered output across runs/platforms. Pin golden cases and
explicitly version any intentional change to this behavior.

## 4. CLI and TUI contract

Planned commands (not yet available):

```sh
nameforge
nameforge tui --data-dir /path/to/local/data
nameforge data import btn --usage irish
nameforge data import btn --usage irish --refresh
nameforge data import btn --saved-pages /path/to/manifest.json
nameforge data list
nameforge data inspect --corpus btn-irish
nameforge generate --corpus btn-irish --gender any --order 2 --count 20 --seed 42
nameforge generate --corpus btn-irish --format json --allow-existing
nameforge version
```

Generation also accepts `--min-length`, `--max-length`, and `--data-dir`.
Default no-argument invocation opens the TUI only with terminal stdin/stdout;
otherwise return usage explaining `generate`, with exit code 2. `--help` works
without a corpus or TTY. `generate` requires an explicit corpus ID. TUI selects
the only installed corpus automatically, or prompts when there are several.

Text generation writes one name per line to stdout, metadata to stderr; JSON
writes one versioned result object including names and reproduction metadata.
Errors go to stderr. Exit codes: 0 success, 1 I/O/data/generation failure,
2 invalid command/options, 130 interrupted. Never emit ANSI in machine output.

TUI state machine: no-data setup -> corpus/filter selection -> generating ->
results, with recoverable error states. No-data setup offers explicit import or
local data instructions; it does not silently start downloading. Network import
and generation run via cancellable Tea commands, never in `View` or blocking
`Update`. Track request IDs so stale completions cannot replace newer results.

Screen: settings panel (corpus, usage, gender, order, count, lengths, novelty),
results list, active seed, corpus count/source, and key hints. Controls:
Tab/Shift-Tab focus, arrows or j/k navigation outside text entry, Enter generate,
r regenerate with a fresh seed, Space toggle favorite, e export, ? help,
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

### M1 — Bootstrap and contracts (pending; no dependencies)

- Create module, CLI skeleton, version/help command and agreed boundary types.
- Add mise tasks: `fmt`, `fmt-check`, `test`, `vet`, `build`, `run`, `check`.
  `check` runs non-mutating format check, tests, vet and build; build writes bin/.
- Add `.github/workflows/ci.yml` using mise and the same project tasks. CI runs
  without a personal corpus or network calls to name sources.
- Acceptance: clean checkout + mise install builds; help/version run; check
  passes; dependencies pinned; task commands documented.
- Commits: `chore(tooling): bootstrap Go module and mise tasks`,
  `feat(cli): add command routing and version output`,
  `ci: verify Go build and tests with mise`.

### M2 — Corpus schema and storage (pending; depends M1)

- Implement DATA.md normalization, schema/version validation, filtering, stable
  serialization/hash, local path precedence, list/inspect and atomic writes.
- Add traceable redistributable test fixtures and their notice; no invented names.
- Acceptance: reject corrupt/unknown schemas, invalid UTF-8 and empty selections;
  normalization/dedup/hash stable; multi-usage/gender semantics tested; failed
  replacement preserves prior data; clean repository after local store use.
- Commits: `feat(corpus): validate and filter versioned name corpora`,
  `feat(data): add local corpus storage and inspection`.

### M3 — Real Behind the Name import (pending; depends M2)

- Implement bounded HTTP/cache and selected-category pagination, DOM parser,
  import reports, saved-page mode and CLI commands per DATA.md.
- Add mocked HTTP tests for pagination, loops, retry, cancellation, changed
  markup, duplicate entries, corrupt cache and transactional failure.
- Acceptance: all DATA.md acceptance items pass; import the three suggested
  categories locally and report actual counts. Do not check imported data in.
- Commits: `feat(source): fetch and cache Behind the Name list pages`,
  `feat(source): parse paginated name lists with provenance`,
  `feat(data): expose validated live and saved-page imports`.

### M4 — Markov engine and generation service (pending; depends M2)

- Implement section 3 independently of TUI/importer. Use sourced fixtures and
  clearly non-name token sequences for small transition-count tests.
- Test boundary tokens, weighted transitions, suffix backoff, deterministic
  ordering, Unicode, length validation, source exclusion, duplicate rejection,
  tiny/empty corpus, impossible requests and cancellation.
- Acceptance: fixed-seed golden cases stable; bounded exhaustion returns the
  documented partial result; model supports independent concurrent requests
  without races; benchmark training and a 100-name batch with corpus size noted.
- Commits: `feat(markov): train deterministic Unicode transition models`,
  `feat(generator): add bounded seeded name generation`.

### M5 — Headless vertical slice (pending; depends M3, M4)

- Implement generation flags, stream formats, metadata, exit codes and docs.
- Acceptance: actual local import -> training -> generation -> JSON/text works
  with network disabled for generation; repeat seed exactly; invalid flags fail
  before expensive work; insufficient diversity is actionable; stdout is clean.
- Commit: `feat(cli): generate reproducible names from local corpora`.

### M6 — Interactive UI (pending; depends M5)

- Implement state machine, settings, results, source/seed display, keyboard
  navigation, cancellable asynchronous work, favorites and exports.
- Test update transitions, stale messages, errors, resize and focus without live
  network access; share CLI generation validation rather than duplicate it.
- Acceptance: actual TTY walkthrough from empty store through import and several
  batches/export; replay displayed seed via CLI; inspect error recovery, narrow
  terminal and no-color mode; Ctrl-C restores terminal during active operations.
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
- Verify artifacts contain no corpus, cached pages or private paths; run a binary
  from outside the checkout with a separately imported local corpus and no Go.
- Document setup/import, offline use, reproducibility, controls, troubleshooting,
  architecture, data sources and dependency notices. Confirm project license
  choice with owner before public release; this does not block personal builds.
- Acceptance: clean-checkout workflow passes, checksummed binaries build, local
  offline generation works, and no runtime dependency beyond binary + data.
- Commits: `ci(release): build checksummed cross-platform binaries`,
  `docs: document installation data import and terminal usage`.

## 6. Execution and handoff

Critical path: M1 -> M2 -> M3/M4 -> M5 -> M6 -> M7. M3 and M4 may be worked on
independently after M2 contracts settle; coordinate go.mod changes. This is a
dependency map for future implementers, not an instruction to spawn agents.

For each handoff record: milestone status, changed contracts, commands/checks and
their results, real import counts where relevant, unresolved limitations, and
next ready work. Never report live-source verification from mocked tests alone.

Key engineering risks: site DOM drift (strict parser and local snapshots), small
corpora (bounded rejection and visible counts), Unicode/casing (NFC/rune tests),
non-deterministic iteration (sorted transitions/goldens), terminal responsiveness
(async commands/cancellation), and dataset updates (hash/versioned provenance).
