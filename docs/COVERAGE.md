# Bundled coverage and M3 acceptance

Source: Faker **v10.6.0**, immutable revision
`2cb04231a6ace91a59ebe577c653f4ec66478ca3`, literal
`src/locales/<locale>/person/first_name.ts` arrays. Full upstream MIT notice is
retained, including inherited notices. Raw SHA-256 values and reviewed category
metadata are in `data/sources.lock.json`; machine-readable extraction accounting
is in `data/quality.json`. No generator outputs are used for training.

Bundle identity:
`81f119c0cbc35c9c28a52f8d2709021cde3c474df4d880730878c39f8bd94498`.

## Observed source coverage

Gender counts overlap only with explicit dual-bucket evidence; here none does.
Generic buckets are unspecified, not unisex. Counts are locale-specific records;
names shared across locales remain separate, and blend training deduplicates them.

| Category | Locale | Distinct | Feminine | Masculine | Unspecified | Letter script | NFC rune range |
| --- | --- | ---: | ---: | ---: | ---: | --- | --- |
| Dutch | nl | 1,085 | 499 | 571 | 15 | Latin | 2–11 |
| English | en | 3,186 | 1,563 | 1,391 | 232 | Latin | 2–11 |
| French | fr | 931 | 435 | 480 | 16 | Latin | 3–12 |
| German | de | 1,145 | 573 | 562 | 10 | Latin | 2–11 |
| Greek | el | 55 | 19 | 36 | 0 | Greek | 4–11 |
| Italian | it | 1,700 | 617 | 1,083 | 0 | Latin | 3–12 |
| Portuguese (Portugal) | pt_PT | 188 | 93 | 95 | 0 | Latin | 3–9 |
| Spanish | es | 227 | 120 | 107 | 0 | Latin | 3–20 |
| Turkish | tr | 1,794 | 392 | 723 | 679 | Latin | 2–17 |
| Arabic (broad source list; optional) | ar | 341 | 10 | 331 | 0 | Arabic | 3–9 |

Total: **10,653 source occurrences, 10,652 accepted, one rejected, zero merged
duplicates**. Dutch `male[571]` contains a control character and is rejected
before trimming; its exact reference and reason appear in the quality report.
No core extraction target is missing. Greek has limited coverage (55 spellings);
Portuguese/Spanish are also smaller than other Latin lists. Broad Arabic has only
ten feminine spellings and does not establish North African regional provenance.
These are source-list labels, not verified cultural identities for individuals.

## Generation smoke (M4 engine, 2026-10-01)

Command:
`mise exec -- go test -v ./internal/generator -run TestBuiltinCategoryGenerationSmoke`.
Algorithm: `markov-v2/nfc-v1/latin-v1/math-rand-v2-pcg`.
Each category uses seed **42**, defaults (20 names, order 2, automatic observed
bounds, gender any, category mode, existing names excluded). Every Latin category
produced **20 distinct novel names**, with identical ordered replay. Generated
names are intentionally not stored in this repository. Rejections below are
length/existing; all other rejection counters were zero.

| Category | Attempts | Length rejections | Existing rejections | Result |
| --- | ---: | ---: | ---: | --- |
| Dutch | 25 | 1 | 4 | 20/20 |
| English | 22 | 0 | 2 | 20/20 |
| French | 24 | 4 | 0 | 20/20 |
| German | 22 | 1 | 1 | 20/20 |
| Italian | 25 | 3 | 2 | 20/20 |
| Portuguese (Portugal) | 28 | 4 | 4 | 20/20 |
| Spanish | 25 | 0 | 5 | 20/20 |
| Turkish | 23 | 0 | 3 | 20/20 |
| Greek | — | — | — | Explicit `unsupported_script` |
| Arabic | — | — | — | Explicit `unsupported_script` |

French + Italian also pass seeded category-order invariance in both category and
blend modes. The original three-record French M4 golden remains unchanged using
its exact provenance-selected subset. Smoke checks measure bounded generation
and replay, not linguistic validity or aesthetic quality. Default Unicode
lowercasing is not Turkish-specific; output capitalizes the first letter after
start/space/hyphen and does not reconstruct culturally specific casing.

Greek and Arabic are inspectable native-script data; generation support is
deliberately deferred by the Latin-only contract. No transliteration or fallback
is supplied. A request selecting all categories therefore encounters an explicit
unsupported-script error. Unisex filtering returns an actionable empty-selection
error for every current category.

## M5 CLI acceptance (2026-10-01)

The engine smoke command above was rerun with the same bundle and algorithm;
every counter in the table is unchanged. `mise exec -- go test -v
./internal/cli -run TestGenerateBuiltinCategorySmoke` also passed through the CLI
for all ten categories: eight default 20-name novel batches and two explicit
unsupported-script failures. Seeded JSON/text replay and selection-order
invariance pass in both French + Italian modes, including repeated category IDs.

`mise exec -- go test -v ./cmd/nameforge` passed on `darwin/arm64`. It builds the
actual executable using pinned mise Go, then runs outside the checkout with an
empty read-only home and no developer tools on PATH. Each runtime subprocess has
macOS sandbox denial of network access and filesystem writes. Both multi-category
modes complete and replay in JSON/text; help, licenses, malformed options and
SIGINT exit 130 pass. The home remains empty. CLI exhaustion tests suppress a
nonempty partial batch and report actionable diversity guidance on stderr.
See [CLI usage](CLI.md) for commands, export schema and stream/exit contracts.

## Rebuild and performance checks

- `mise run data:fetch`, `mise run data:build`, `mise run data:verify` passed.
- Fresh independent cache: `mise exec -- go run ./cmd/corpus-build fetch --cache
  .local/faker-clean`, followed by `verify --rebuild --cache .local/faker-clean`,
  reproduced all public artifact bytes, including the notice and quality report.
- `verify --cache .local/absent-cache` passed without any raw cache.
- `mise run check`, `mise exec -- go test -race ./...`, and
  `mise exec -- go mod verify` passed. Parser tests reject unknown expressions,
  malformed literals and invalid UTF-8; extraction tests cover normalization,
  merged bucket evidence, invalid records and generic semantics; fetch/build tests
  cover checksum mismatch, missing files, bounded retries and cancellation.

Benchmarks on Apple M2 Max (`darwin/arm64`), run with
`mise exec -- go test ./internal/markov ./internal/generator -run '^$' -bench Builtin -benchmem`:

| Operation | Input size | Measured ns/op | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: |
| Order-2 training, all bundled locale records | 10,652 | 12,495,744 | 3,044,653 | 145,329 |
| 100 model samples, trained union | 10,652 | 53,190 | 1,176 | 100 |
| 100 distinct novel French names, including request preparation/training | 931 | 13,001,184 | 15,916,013 | 103,237 |

The raw sampler benchmark includes native-script training and does not measure
Latin-only validation or uniqueness. The end-to-end benchmark includes request
metadata/hashing and rejects existing/duplicate names. Results are a baseline,
not a cross-machine performance guarantee.
