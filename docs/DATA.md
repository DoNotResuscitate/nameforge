# Bundled data and category design

## Decision (updated 2026-10-01)

Use **Faker's static locale name lists** for eight embedded packs. M5a replaces
the native-script Greek/Arabic packs with Wikipedia's supplied Latin/Greek name
pairs (CC BY-SA 4.0) and Wikidata's supplied Latin name statements (CC0). See
[ROMANIZED.md](ROMANIZED.md) for pinned sources and the reviewed selection. This
replaces mandatory Behind the Name imports. The user's goal is TTRPG names,
multiple selectable categories, and a distributable binary that works offline
immediately. Prioritize Mediterranean and Western European names, plus Turkish;
North African coverage is desirable when accurately sourced. All training names
must come from existing resources, never agents.

### Sources evaluated

| Source | Redistribution / format | Fit and decision |
| --- | --- | --- |
| [faker-js/faker](https://github.com/faker-js/faker) | MIT, static TypeScript arrays, more than 70 locales advertised | Primary: broad multilingual coverage, maintained, easy to extract and embed with notices. Locale count is not a promise of that many usable name lists. |
| [Wikidata](https://www.wikidata.org/wiki/Wikidata:Licensing) | Structured data CC0, JSON/SPARQL | Later enrichment for historical/name-language categories. Requires explicit queries, entity validation and coverage review; not a ready-made curated name pack. |
| [Behind the Name](https://www.behindthename.com/names/list) | HTML | Category-selection UX reference and possible later import adapter. |
| [smashew/NameDatabases](https://github.com/smashew/NameDatabases) | Plain text, repository Unlicense | Not selected: `NamesDatabases/credits.txt` lists mixed web sources including Behind the Name and says only “so far as I know” lists are not copyrighted. The repository license alone does not resolve that provenance. |
| [aruljohn/popular-baby-names](https://github.com/aruljohn/popular-baby-names) | MIT, SSA-derived CSV/JSON | Easy but US popularity categories do not meet the desired breadth alone. |

Verified Faker stable release: **v10.6.0**, commit
`2cb04231a6ace91a59ebe577c653f4ec66478ca3`.

- [License at the release](https://github.com/faker-js/faker/blob/v10.6.0/LICENSE)
- [Locale documentation](https://fakerjs.dev/guide/localization.html)
- Data path: `src/locales/<locale>/person/first_name.ts`.
- Preserve the complete upstream LICENSE, including inherited faker.js notices,
  in committed assets and release archives; expose it via `nameforge licenses`.
- These are established lists used by a fake-data library. Use the literal name
  data; do not call its random-name API to manufacture a training corpus.

## Initial category coverage

Target these real source arrays, confirmed present at the pinned release:

| Category ID | UI label | Source locale |
| --- | --- | --- |
| `turkish` | Turkish | `tr` |
| `greek` | Greek | `el` |
| `portuguese-pt` | Portuguese (Portugal) | `pt_PT` |
| `dutch` | Dutch | `nl` |
| `german` | German | `de` |
| `french` | French | `fr` |
| `spanish` | Spanish | `es` |
| `italian` | Italian | `it` |
| `english` | English | `en` |
| `arabic` | Arabic (broad source list; optional) | `ar` |

This is an initial extraction target, not a measured coverage claim. M3 must
publish actual distinct-name counts, available gender labels, script inventory,
length ranges and generation smoke results for each category. Add other genuine
locale arrays when reviewed; exclude novelty locales such as `en_BORK`. Missing
data must not silently fall back to English. Do not label `en_IE` as Irish Gaelic
or modern Norwegian as Old Norse. Unsupported categories are absent, not filled
with invented or mislabelled data.

The nine M3 core targets are French, Spanish, Italian, Portuguese (Portugal), Greek,
Turkish, German, Dutch and English. Arabic was the optional tenth extraction pack;
M5a requires romanized replacements for both Greek and Arabic before UI work.
Prioritize depth and useful Markov output for these targets over worldwide category counts.
If a core target fails quality/coverage checks, report the gap and investigate
another licensed source rather than silently replacing its culture or language.

The `cy` locale exists but has no `person/first_name.ts` at the pinned revision;
Welsh is therefore expansion work, not a promised bundled category. Likewise
Irish Gaelic, Breton, Basque, Catalan and historical Mediterranean lists need
separate source validation. Northern/Eastern European and worldwide packs can
follow after the primary region works well.

Neither the original generic `ar` list nor the replacement Wikidata pack establishes Moroccan, Algerian, Tunisian, Egyptian
or Amazigh/Berber provenance. Label it Arabic, not North African. Region-specific
North African packs are a later data-research task; broad Arabic can be used now
only under its accurate source label.

Faker does not provide Behind the Name's full ancient/mythology/fantasy taxonomy.
Those remain explicit expansion work. Blending existing categories can provide
TTRPG naming styles without inventing a training dataset or claiming fictional
cultures are real-world ones.

## Build-time extraction, not runtime acquisition

1. Add `data/sources.lock.json` with repository URL, exact commit, chosen paths,
   raw SHA-256 checksums, license path/hash and extractor/schema versions.
2. A Go maintenance command under `cmd/corpus-build/` fetches only pinned files
   on an explicit `mise run data:fetch`, using timeouts, bounded retry, caching
   under `.local/`, and checksum validation. No fetch in ordinary build/startup.
3. `mise run data:build` extracts cached static arrays and emits canonical assets.
   Parse the required TypeScript literal subset (comments, quoted strings with
   escapes, arrays, object keys and export wrapper) with a small tested lexer /
   parser. Fail on unknown expressions/imports/spreads; never eval arbitrary
   code or scrape string-looking fragments with regex. If a selected upstream
   file requires another construct, support it explicitly or report it.
4. Map upstream `male`/`female` labels; `generic` means unspecified, **not**
   evidence of unisex usage. Preserve original bucket membership in provenance.
   A spelling explicitly appearing in both male and female arrays can support
   the `unisex` filter. Missing labels are not guessed.
5. NFC-normalize and validate, deduplicate stably, and produce quality reports
   with source counts, accepted/rejected counts and reasons. Keep names, not
   prefixes, suffixes, job titles, full-name templates or synthetic outputs.
6. Commit canonical corpus assets and the full upstream notices under
   `internal/corpus/assets/`; load them read-only with `//go:embed` and `embed.FS`.
   Do not embed a broad filesystem glob that can include personal data.
7. `mise run data:verify` checks committed schema, hashes, coverage and notice
   presence without network. A maintenance rebuild from the locked raw files
   must reproduce canonical asset bytes. Data refreshes are explicit atomic
   commits, not an incidental effect of build or release.

No Node/TypeScript runtime is required with this extraction approach. If an
implementation decision adds a developer-only tool, pin it in mise and document
why; release users still need only the Go binary.

## Corpus format v1

Assets contain `manifest.json`, `categories.json` and `names.jsonl`, plus notices.
Each UTF-8 JSONL record has:

- `id`: stable source ID + locale + normalized-name hash.
- `name`: source display spelling, trimmed and NFC-normalized.
- `categories`: sorted unique category IDs.
- `genders`: sorted subset of `masculine`, `feminine`; empty is unspecified.
- `source_refs`: source revision, file path, bucket and entry index for each
  original occurrence; preserve provenance when duplicates merge. Optional
  `statement_id`, `native_name` and `evidence` retain Wikimedia statement/row
  identity, untouched native associations and statement IDs supporting classification.

Each category has stable ID, display label, group (`language-region` initially;
allow `historical`, `mythological`, `fictional` later), source locale, observed
scripts, supported genders and counts. Script metadata is not a transliteration.
Category mappings are reviewed metadata, not inferred from a person's identity.

Manifest: schema/source revision, source paths/checksums, license references,
extractor/normalization versions, record counts and SHA-256 of canonical records
and category metadata. Bundle identity hashes names + categories + normalization
version; timestamps are excluded from reproducibility hashes.

Source files may additionally carry `revision` and `url`. A record reference must
match its file's explicit revision; when omitted it matches the manifest's default
`source_revision` (the original Faker revision). This backward-compatible v1
extension permits multiple source revisions without rewriting the other eight
packs' IDs or references. Greek indices count numbered rows within the named
source section; its `statement_id` identifies section/row/variant. Arabic indices
address the original `claims.P1705` array, with exact statement IDs retained.
Replacement IDs are `wikipedia:el:<normalized-spelling SHA-256>` and
`wikidata:ar:<normalized-spelling SHA-256>`; existing Faker IDs remain unchanged.

For training, deduplicate NFC/lowercase spellings within each selected category;
in blend mode also deduplicate across categories. Do not strip accents or
transliterate. Retain letters, combining marks, apostrophes, hyphens and internal
spaces. Reject invalid UTF-8 and control characters with explicit reports.
Source-meaningful format characters in non-Latin text need an explicit tested
policy rather than silent stripping. Record Turkish casing and other
language-specific limitations of the default Unicode lowercase mapping.

The current model and generator identify duplicate spellings with NFC followed
by Go's default Unicode `strings.ToLower`, not locale-sensitive case folding.
This may not match Turkish orthographic case equivalence for `I`, `İ`, `ı` and
`i`; spellings are never transliterated. The algorithm version covers this
behavior so a future normalization change can be replayed explicitly.

## Script and usability policy

The current generator emits Latin-script output only: Latin letters, supported
combining diacritics, and spaces/apostrophes/hyphens. Do not transliterate Greek,
Arabic or other source scripts. Categories without a Latin script profile return
an explicit unsupported-script error, and sampled candidates containing other
scripts or characters are rejected. Mixed-script source categories may still be
listed, but cannot cause non-Latin output. The original Faker Greek and Arabic
arrays cannot serve as romanized training lists. M5a replaces those training packs
with source-provided Latin spellings. Two exact Faker native-script records remain
only in `internal/generator/testdata/nonlatin.json` for rejection tests and are not
embedded in the runtime. Generation policy and algorithm version are unchanged.

Category mode keeps each candidate within one category. Blend mode requires
matching script profiles for MVP (e.g. Latin + Latin); reject incompatible
profiles with an explanation. Preserve source-internal mixed scripts rather
than arbitrarily splitting source records. Test Latin diacritics, combining
marks, and separators with sourced fixtures. Non-Latin generated-text rendering
is deferred with non-Latin generator support.

Use category-derived automatic length bounds (observed minimum/maximum capped
at 64) unless the user explicitly sets bounds. The character-level rune model
is an MVP approximation; document Unicode casing and normalization behavior.

## M5a romanized dataset requirements (before M6 UI)

Both `greek` and `arabic` must be usable for Latin-only generation before UI work.
Their original Faker packs used Greek/Arabic scripts; M5a selects the licensed
Wikipedia/Wikidata sources documented in [ROMANIZED.md](ROMANIZED.md).
Romanized means the Latin spelling is supplied by the external
dataset, not generated by Faker, an agent, or a project transliteration step.

- Preserve category IDs and label the replacement packs as Greek (romanized) and
  Arabic (romanized), with the actual source scope and Latin script profile.
  Generic Arabic remains broad, without inferred North African regional coverage.
- Review and document source romanization conventions, provenance and coverage.
  Retain source-supported gender labels only; an unlabeled romanized entry is
  unspecified. Do not transfer gender evidence from an unrelated spelling/list.
- Pin source revisions, exact files/URLs and raw checksums in the source lock;
  retain per-record references and complete notices. Extend the maintenance
  extractor/verification boundaries for multiple sources while keeping runtime
  generation offline. Keep the other eight Faker-derived categories unchanged.
- Apply the existing NFC/lowercase deduplication and source exclusion rules to
  supplied Latin spellings, preserving accents. Original-script associations may
  be retained as traceable provenance if supplied; training/output use romanized
  spellings. Report rejected records and measured gender/script/length coverage.
- Require enough real data for each replacement category to generate 20 distinct
  novel names at defaults and seed 42. All ten packs must have compatible Latin
  profiles so explicit all-category generation succeeds in both modes, with
  deterministic replay and no non-Latin output. Tests continue rejecting truly
  unsupported source scripts; the generator's accepted character policy stays
  Latin-only.
- Rebuild from pinned caches byte-for-byte, verify committed assets offline, and
  expose every source's complete notice via `licenses`. Update coverage and
  first-run binary acceptance for Greek, Arabic and all categories. A changed
  corpus hash is expected; the sampling/normalization algorithm remains unchanged.

## Optional personal sources (later, not on the MVP critical path)

Keep a future `btn:<usage>` namespace separate from `builtin:<category>`. A later
Behind the Name adapter can follow selected usage-list pagination, cache pages
and parse entry headers only (not name links in definitions), supporting
saved-page imports and atomic updates.
The inspected Irish list has pagination, diacritics, numbered disambiguators and
multiple usage/gender labels. Keep this HTML/data under ignored local storage.

Optional runtime directory precedence remains `--data-dir`,
`NAMEFORGE_DATA_DIR`, then `filepath.Join(os.UserConfigDir(), "nameforge")`.
Built-ins load even with an empty/unwritable home directory; never extract them
to a writable cache as a prerequisite. Local packs cannot replace built-in IDs.

## Historical M3 extraction and measured coverage

M3 replaced the M2 three-record French schema fixture with all ten reviewed
locale arrays: 10,652 accepted locale-specific records. Its original source lock
is now retained byte-for-byte as `data/faker.lock.json`; canonical assets and the complete upstream notice are
under `internal/corpus/assets/builtin/`. `data/quality.json` records every source
bucket count, accepted/rejected/merged occurrence counts, gender counts, observed
letter scripts and NFC rune-length ranges. [COVERAGE.md](COVERAGE.md) summarizes
coverage, fixed-seed M4 smoke checks, benchmarks and remaining usability limits.

One Dutch occurrence (`male[571]`) is rejected for a control character. Extraction
checks controls before trimming, rejects format characters explicitly, and
accepts Unicode letters/marks with supported internal separators. It never strips
format characters or transliterates. Unknown source syntax and a category with
no accepted names are hard errors. Reports retain rejected references/reasons.
Generic is unspecified; no accepted spelling in these pinned arrays appears in
both male/female buckets, so all current categories have zero unisex matches.

Deduplication uses NFC and default Unicode lowercase within each locale. Records
are kept separate across locales so gender evidence does not leak between source
categories. Each stable ID is `faker:<locale>:<SHA-256 of lowercase NFC spelling>`.
Display spelling is the first accepted source occurrence in sorted bucket order
(`female`, `generic`, `male`), then source index order. All merged occurrences
remain in provenance. Training/blending deduplication follows the existing engine
contract. Corpus schema and normalization versions are unchanged.

The M3 verifier checks canonical schema/content hashes, exact reviewed
targets and source checksums, notice bytes against its locked checksum, script
metadata, occurrence accounting and the canonical quality report. It needs no raw
cache. After explicit `data:fetch`, `corpus-build verify --rebuild` re-extracts the
checksum-validated raw cache and compares every public artifact byte-for-byte.
Normal tests use embedded sourced data or clearly non-name parser/algorithm tokens.
The M4 three-record golden still selects the original sourced French bucket/index
references from the full bundle, so expanding data does not rewrite that golden.

## M5a extraction and measured coverage

The current bundle has **10,851 records**: the same 10,256 records in the eight
remaining Faker packs, 486 Greek spellings and 109 Arabic spellings. All ten
categories have Latin script profiles. Greek has no source-supported gender
labels; Arabic has 22 feminine and 87 masculine spellings. No current spelling has
both gender labels. Corpus/schema and generation/normalization versions remain
unchanged; the maintenance extractor is `multisource-static-v1` and the changed
bundle identity is reported in `data/quality.json` and [COVERAGE.md](COVERAGE.md).

The root source lock checksum-pins the original Faker lock and every supplemental
revision URL, raw checksum and full license text. Maintenance first reconstructs
the original Faker assets in memory, preserves eight packs unchanged and produces
the two sourced native-script test records, then replaces the Greek/Arabic runtime
training packs. Native-script Faker files remain locked/fetched for this fixture
and reproducibility, never as romanized training input.

Supplemental extraction keeps exact source spellings; all variants are external,
not authored or transliterated. The quality report accounts for 10,920 occurrences:
10,863 accepted, 57 rejected and 12 merged, yielding 10,851 distinct category records.
Offline verification checks revision-aware provenance, every source file's use,
contiguous Faker/Greek row accounting, unique occurrences, reviewed exclusions,
Latin-only training, catalog/gender semantics, hashes, full notices and quality.
`verify --rebuild` additionally compares every artifact byte, including the
non-embedded test fixture, against checksum-validated raw caches. Normal build,
tests and runtime never query upstream sources.
