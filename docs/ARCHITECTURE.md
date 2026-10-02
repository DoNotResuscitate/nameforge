# Architecture and reproducibility

Nameforge is one Go executable. Normal startup reads an explicitly embedded,
redistributable asset directory; it never downloads training data or extracts it
to disk. Built-ins work with an empty, unwritable home. Filesystem writes occur
only at an explicit export destination. There is no runtime model cache or service.

## Boundaries

```text
cmd/nameforge -> internal/cli -> internal/tui
                        \          /
                    internal/generator
                       /         \
             internal/corpus   internal/markov

CLI/TUI -> internal/export -> internal/store (explicit atomic exports)
CLI/TUI -> internal/legal (embedded application/dependency/data notices)
```

- `cmd/nameforge`: process context, SIGINT handling and exit status.
- `internal/cli`: stdlib flag parsing, terminal detection, clean machine streams.
- `internal/tui`: Bubble Tea state machine, focus and rendering. Generation and
  export run in cancellable commands with request IDs; stale replies are ignored.
- `internal/corpus`: strict versioned schema, source references, Unicode
  normalization, category/gender selection, canonical hashes and read-only embedding.
- `internal/markov`: immutable character-level models. No terminal, filesystem,
  network, or shared RNG state; models support independent concurrent requests.
- `internal/generator`: shared validation, category/blend selection, sampling,
  novelty/uniqueness checks, bounded rejection, cancellation and replay metadata.
- `internal/export` / `internal/store`: versioned JSON/text and atomic publication;
  no-clobber exports require explicit overwrite confirmation.
- `internal/legal`: full GNU GPLv3, pinned dependency notices and corpus attribution,
  available through `licenses` and the interactive `l` screen without a network.

Maintenance commands are separate executables, excluded from runtime releases:
`cmd/corpus-build` fetches locked sources only when explicitly requested;
`cmd/notices` derives legal texts from pinned module/toolchain files;
`cmd/release-build` cross-compiles, audits and packages explicit public files.

## Deterministic generation

Normalize training spellings to NFC, compare with Go's default Unicode lowercase,
and deduplicate. Characters and length limits are Unicode runes. Sort categories,
training spellings and successor tokens before sampling. Every request owns a
`math/rand/v2` PCG seeded with `(seed, seed XOR 0x9e3779b97f4a7c15)`.
No iteration over an unordered Go map controls random choices.

An order-1..4 model uses internal START/END tokens and counted suffix contexts.
Sampling backs off only when a longer context is absent. Reject malformed
separators, non-Latin characters, out-of-bound NFC lengths, repeated batch names,
and (by default) exact selected training names. Never truncate, transliterate or
substitute fallback names. Attempts stop at `max(1000, count*200)` and each raw
sample is capped at 128 runes. Cancellation is checked between attempts.

Category mode chooses each selected category equally per output slot and retries
inside that category. Blend mode learns from the deduplicated union, where larger
lists contribute more transitions; attribution identifies every contributing
category as a hybrid style. Empty filtered categories are explicit errors.
Unspecified gender is never inferred as unisex.

Replay requires the same seed, options, algorithm version and corpus hash. The
algorithm is `markov-v2/nfc-v1/latin-v1/math-rand-v2-pcg`; changes to these semantics
must receive a new version. Corpus refreshes change the bundle hash. Tests retain
a sourced French golden independently of the current expanded bundle, and pin
seeded all-category replay and selection-order invariance. See [CLI.md](CLI.md)
for metadata and [DATA.md](DATA.md) for normalization/provenance contracts.

## Build and verification

Go 1.27.1 and actionlint 1.7.12 are pinned in mise. Runtime Go libraries are pinned
in `go.mod`/`go.sum`; actionlint is development-only workflow validation. Normal
checks and corpus verification do not contact name sources. CI runs checks and
race tests natively on macOS arm64 and Linux amd64/arm64, with public-token parser and
normalizer fuzz smoke checks on Linux amd64. Actual executable tests exercise CLI
and keyboard-driven PTY workflows outside the checkout with an empty home and no
developer tools on the subprocess PATH. CI enforces network denial using macOS
sandboxing or an empty Linux network namespace.

Release builds disable CGO and use trimpath. Deterministic archive ordering,
owner/mode fields and revision-derived timestamps avoid local path/user leakage.
Merging to main calculates a stable Conventional Commit version and creates its
tag at the exact merged revision. Release CI compares independent build checksums, rebuilds vendored corresponding
source without module downloads, then tests extracted macOS arm64 and both Linux
binaries on native runners before automatic GitHub publication. Intel macOS is
cross-built only and is explicitly labeled that way in release verification notes.
The same run handles tag creation and
publication using `GITHUB_TOKEN`; no recursive tag workflow is needed.
`cmd/release-version` previews versions without mutation. See [RELEASE.md](RELEASE.md).
