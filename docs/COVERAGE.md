# Bundled coverage and M5a acceptance

Eight packs use Faker **v10.6.0**, immutable revision
`2cb04231a6ace91a59ebe577c653f4ec66478ca3`, literal
`src/locales/<locale>/person/first_name.ts` arrays. Full upstream MIT notice is
retained, including inherited notices. Raw SHA-256 values and reviewed category
metadata are in `data/faker.lock.json`. Greek uses Wikipedia revision 1377658475
(CC BY-SA 4.0); Arabic uses 119 revision-pinned Wikidata entities (CC0).
Supplemental URLs/checksums and the Faker lock checksum are in
`data/sources.lock.json`; machine-readable extraction accounting
is in `data/quality.json`. No generator outputs are used for training.

Bundle identity:
`36cd9b6d35049fcd815b233ba12f1565e852157438b5333af7621526a41a4da9`.

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
| Greek (romanized; ancient and modern) | el | 486 | 0 | 0 | 486 | Latin | 2–13 |
| Italian | it | 1,700 | 617 | 1,083 | 0 | Latin | 3–12 |
| Portuguese (Portugal) | pt_PT | 188 | 93 | 95 | 0 | Latin | 3–9 |
| Spanish | es | 227 | 120 | 107 | 0 | Latin | 3–20 |
| Turkish | tr | 1,794 | 392 | 723 | 679 | Latin | 2–17 |
| Arabic (romanized; broad Wikidata list) | ar | 109 | 22 | 87 | 0 | Latin | 3–13 |

Total: **10,920 source occurrences, 10,863 accepted, 57 rejected, 12 merged
occurrences, 10,851 distinct category records**. Dutch `male[571]` contains a control character and is rejected
before trimming; its exact reference and reason appear in the quality report.
Greek contributes 526 occurrences (495 accepted, 31 rejected, nine merged);
Arabic contributes 137 class/spelling occurrences (112 accepted, 25 rejected,
three merged). Reviewed exclusions and source scope are documented in
[ROMANIZED.md](ROMANIZED.md). Greek's mixed ancient/modern examples fit the
intended early-modern fantasy setting, without claiming exact historical usage.
Greek lacks gender evidence; Arabic is small and does not establish regional provenance.
Portuguese/Spanish are also smaller than other Latin lists.
These are source-list labels, not verified cultural identities for individuals.

## Generation smoke (M5a corpus, unchanged M4 engine, 2026-10-01)

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
| Greek | 23 | 2 | 1 | 20/20 |
| Arabic | 28 | 1 | 7 | 20/20 |

French + Italian also pass seeded category-order invariance in both category and
blend modes. The original three-record French M4 golden remains unchanged using
its exact provenance-selected subset. Smoke checks measure bounded generation
and replay, not linguistic validity or aesthetic quality. Default Unicode
lowercasing is not Turkish-specific; output capitalizes the first letter after
start/space/hyphen and does not reconstruct culturally specific casing.

Greek, Arabic and all-category requests now succeed with source-supplied Latin
training spellings. Native Faker examples remain only as non-embedded regression
fixtures and still return `unsupported_script`. All-category replay, novelty,
uniqueness, compatible scripts and attribution pass in category and blend modes,
through both CLI formats. Greek's missing gender evidence remains an explicit
filtered-selection error; it is never inferred or filled from another source.
Unisex filtering returns an actionable empty-selection error for every current
category.

## M5a acceptance (2026-10-01)

- `mise run check`, `mise run data:verify`, `mise exec -- go test -race ./...`
  and `mise exec -- go mod verify` passed on `darwin/arm64`.
- `mise run data:fetch`, `mise run data:build` and
  `mise exec -- go run ./cmd/corpus-build verify --rebuild` passed. Re-extraction
  from the pinned raw cache reproduced every artifact byte, including all notices,
  the quality report and the non-embedded native-script fixture.
- `mise exec -- go run ./cmd/corpus-build verify --cache .local/absent-cache`
  passed without a raw cache or network access.
- All ten individual categories produce/replay 20 distinct novel Latin-only names
  at seed 42/defaults. The original sourced French algorithm golden is preserved.
- `TestAllCategoriesLatinNovelReplayAndAttribution` passed. Category mode used
  24 attempts (two length, two existing rejections); blend mode used 21 attempts
  (one existing rejection). Both produced 20 novel, unique names with clean
  text/JSON streams, complete metadata and correct attribution. Ordered seeded
  replay and reversed explicit-category selection also passed.
- `TestBinaryHeadless` runs the actual executable outside the checkout with an
  empty read-only home, no developer tools on PATH, and macOS sandbox denial of
  network access and filesystem writes. Greek, Arabic, all-category and French +
  Italian requests succeed/replay in both modes and formats. Help, notices,
  invalid options and SIGINT still pass. The race run also executes this test.
  Final uncached command: `mise exec -- go test -count=1 -v ./cmd/nameforge` passed.
- Source tests cover revision/claim validation, Latin-only selection, missing
  gender evidence, supplied variants, Unicode NFC/deduplication, rejected unknown
  wikitext, pinned checksums, missing caches, retries/cancellation and corrupt
  notices/reports. Native-script errors remain covered by sourced Faker fixtures.

Limitations: Greek's source has no gender labels; all current unisex pools are
empty. Both replacements use heterogeneous source spellings rather than a single
romanization standard. Wikipedia/Wikidata are community-maintained and generally
not frequency data; Arabic's Latin-statement citations are sparse. Native checks
here are macOS arm64, with OS-enforced network denial claimed only on macOS.
No generated batches or raw research pages are committed. Next ready milestone: M6.

## Historical M5 CLI acceptance (2026-10-01, original Faker bundle)

At M5 acceptance, the original Faker bundle had eight Latin and two native-script
packs. The eight Latin counters above remain unchanged after M5a. `mise exec -- go test -v
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

## Historical M3 rebuild and performance checks

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

M3 baseline benchmarks on Apple M2 Max (`darwin/arm64`), using the original
10,652-record Faker bundle (hash
`81f119c0cbc35c9c28a52f8d2709021cde3c474df4d880730878c39f8bd94498`), run with
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
