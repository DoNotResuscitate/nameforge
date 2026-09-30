# Data acquisition and provenance

## Source decision (2026-09-30)

Primary source: https://www.behindthename.com/names/list

The user states that they have acquired email permission and that this is a
personal tool. Treat personal acquisition/use as authorized; do not ask agents to
obtain permission again. Keep the correspondence private. Public site policies
are at https://www.behindthename.com/info/terms and
https://www.behindthename.com/info/copyright. The email's exact wording is not in
this repository; do not infer permission to redistribute the corpus. Ship the
binary separately, with local data import supported on each installation.

The index provides usage categories, historical/mythological categories, and
gender navigation. A checked example, `/names/usage/irish`, has multiple pages,
diacritics, entries with multiple usages/genders, and numbered disambiguators.
It also contains many name links inside explanatory prose. Those are **not**
additional list records. DOM selectors must target list entry headers only.

## Import scope and behavior

1. Start with an explicit usage slug, initially `irish`. Add `english` and
   `ancient-scandinavian` as acceptance datasets once the importer works. These
   are suggested starting categories, not a hardcoded limit on supported lists.
2. Discover usage labels/URLs from the index on an explicit import/discovery
   action. Cache the index. Do not crawl all categories automatically.
3. Fetch the selected list and follow its actual pagination links. Stay on the
   expected HTTPS host and selected usage path; detect loops and enforce a
   configurable page limit (default 100). Hitting that limit is an incomplete
   import error, not success. Do not visit each name's detail page in MVP.
4. Use Go HTTP with timeouts, cancellation, an identifiable user agent,
   concurrency 1, at least two seconds between requests, and up to three retries
   for transient network/5xx failures. Honor Retry-After for 429. Stop on 401/403
   or challenge pages and report the failure; accept saved HTML for local import.
5. Cache responses with URL, fetched time, ETag/Last-Modified when available,
   and SHA-256. Reuse cache by default; `--refresh` explicitly revalidates it.
6. Parse HTML with `golang.org/x/net/html` (not regex). Capture entry name,
   canonical entry URL, usage labels/slugs, and source gender labels. Remove
   disambiguation markers structurally, preserving the URL as source identity.
   Do not import definitions, meanings, unrelated navigation, or prose links.
7. Validate every page. A successful HTTP response containing zero recognizable
   entries, unexpected layout, or broken pagination is an error. Preserve the
   previous working corpus until all pages validate.
8. Normalize and merge deterministically, emit an import report, and replace the
   corpus directory atomically. Preserve the prior snapshot on interruption.

Provide a saved-page import mode with an explicit manifest of original URLs,
local file paths, and page order. It uses the same parser and validator. Do not
claim a partial saved-page set is a complete category; record completeness.

## Local corpus format v1

Use a directory with `manifest.json` and `names.jsonl`. Each UTF-8 JSONL record:

- `id`: source ID plus canonical entry path (stable, independent of fetch time).
- `name`: original display spelling, trimmed and NFC-normalized.
- `usages`: sorted unique source usage slugs.
- `genders`: sorted subset of `masculine`, `feminine`; empty means unspecified.
  An entry with both source labels has both values.
- `source_url`: canonical HTTPS entry URL.

Manifest fields: `schema_version`, `source_id`, source homepage, usage selections,
source permission note (`user-reported email permission, personal use`), fetched
page URLs/timestamps/hashes, importer version, normalization version, record
count, completeness, and SHA-256 of canonical `names.jsonl` bytes. Keep the corpus
content hash independent of timestamps for deterministic generation/cache keys.

Deduplicate source entries by stable ID and merge their metadata. For training,
deduplicate normalized spellings across selected entries so repeated category
membership does not multiply weight. Preserve distinct source records on disk.
Use Unicode lowercase + NFC as the training/comparison key; do not strip accents
or transliterate. Preserve letters, combining marks, apostrophes, hyphens and
internal spaces. Reject control characters and invalid UTF-8, and report rejects.
Do not silently make up missing labels or repair names by guessing.

## Storage and fixtures

Runtime directory precedence: `--data-dir`, `NAMEFORGE_DATA_DIR`, then
`filepath.Join(os.UserConfigDir(), "nameforge")`. Store corpora under `corpora/`
and fetched pages under `cache/`. Repository-local experiments use `.local/`.

Committed name-bearing fixtures must come from an independently redistributable
source with its notice and exact source revision. A researched option is
https://github.com/aruljohn/popular-baby-names, an SSA-derived list with an MIT
license. Inspected revision: `91bb24b84af1ff5bfa5233ea3431534f458add9e`;
`2025/girl_boy_names_2025.csv` has `Rank,Girl Name,Boy Name` columns. This offers
test names, not equivalent Behind the Name usage metadata. Preserve its LICENSE
when extracting fixtures. Do not silently substitute this source for the user's
selected Behind the Name corpus.

Test DOM parsing with structural fixtures and non-name placeholder tokens; run
actual source-page integration checks against ignored local snapshots. Do not
commit Behind the Name HTML or corpus-derived model files by default.

## Data milestone acceptance

- Import all pages of one real category; report page/record/rejection counts.
- Trace sampled imported names back to their source entry headers.
- Verify diacritics, multiple labels, disambiguators and pagination handling.
- Rebuild from the same cache and get byte-identical canonical records/hash.
- Fail a later-page fetch/parse and demonstrate the old corpus still loads.
- Repeat from saved pages with the network unavailable.
