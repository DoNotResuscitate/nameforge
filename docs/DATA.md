# Bundled data and category design

## Decision (updated 2026-09-30)

Use **Faker's static locale name lists** for the initial embedded dataset. This
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
| [Behind the Name](https://www.behindthename.com/names/list) | HTML, user has permission for personal use | Excellent taxonomy and optional personal extension; not the default bundled source. |
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

The nine core targets are French, Spanish, Italian, Portuguese (Portugal), Greek,
Turkish, German, Dutch and English. Arabic is an optional tenth pack. Prioritize
depth and useful Markov output for these targets over worldwide category counts.
If a core target fails quality/coverage checks, report the gap and investigate
another licensed source rather than silently replacing its culture or language.

The `cy` locale exists but has no `person/first_name.ts` at the pinned revision;
Welsh is therefore expansion work, not a promised bundled category. Likewise
Irish Gaelic, Breton, Basque, Catalan and historical Mediterranean lists need
separate source validation. Northern/Eastern European and worldwide packs can
follow after the primary region works well.

The generic `ar` list does not establish Moroccan, Algerian, Tunisian, Egyptian
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
  original occurrence; preserve provenance when duplicates merge.

Each category has stable ID, display label, group (`language-region` initially;
allow `historical`, `mythological`, `fictional` later), source locale, observed
scripts, supported genders and counts. Script metadata is not a transliteration.
Category mappings are reviewed metadata, not inferred from a person's identity.

Manifest: schema/source revision, source paths/checksums, license references,
extractor/normalization versions, record counts and SHA-256 of canonical records
and category metadata. Bundle identity hashes names + categories + normalization
version; timestamps are excluded from reproducibility hashes.

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
listed, but cannot cause non-Latin output. Greek and Arabic arrays remain
accurately labeled source data, not romanized lists; script expansion requires
a deliberate future generator change.

Category mode keeps each candidate within one category. Blend mode requires
matching script profiles for MVP (e.g. Latin + Latin); reject incompatible
profiles with an explanation. Preserve source-internal mixed scripts rather
than arbitrarily splitting source records. Test Latin diacritics, combining
marks, and separators with sourced fixtures. Non-Latin generated-text rendering
is deferred with non-Latin generator support.

Use category-derived automatic length bounds (observed minimum/maximum capped
at 64) unless the user explicitly sets bounds. The character-level rune model
is an MVP approximation; document Unicode casing and normalization behavior.

## Optional personal sources (later, not on the MVP critical path)

Keep a future `btn:<usage>` namespace separate from `builtin:<category>`. The
user has permission for personal Behind the Name use; do not prompt again. No
permission to redistribute its corpus is assumed. A later adapter can follow
selected usage-list pagination, cache pages and parse entry headers only (not
name links in definitions), supporting saved-page imports and atomic updates.
The inspected Irish list has pagination, diacritics, numbered disambiguators and
multiple usage/gender labels. Keep this HTML/data under ignored local storage.

Optional runtime directory precedence remains `--data-dir`,
`NAMEFORGE_DATA_DIR`, then `filepath.Join(os.UserConfigDir(), "nameforge")`.
Built-ins load even with an empty/unwritable home directory; never extract them
to a writable cache as a prerequisite. Local packs cannot replace built-in IDs.

## M2 schema fixture

The initial embedded bundle is a validation fixture, not the M3 training corpus.
It contains three literal entries from the pinned Faker `fr/person/first_name.ts`
arrays: `female[0]` (`Abdonie`), `male[0]` (`Aaron`), and `generic[0]` (`Alix`).
Their record references preserve those exact buckets and indices. The raw source
SHA-256 is recorded in the fixture manifest, and the complete upstream Faker
license is embedded under `internal/corpus/assets/builtin/licenses/`. Its displayed
category label explicitly marks it as a schema fixture so the count is not read as
coverage. M3 replaces this tiny bundle with the reproducibly extracted category
packs and coverage report.
