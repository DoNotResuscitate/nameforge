# Interactive Nameforge

Install via [RELEASE.md](RELEASE.md#install-a-standalone-binary), or build
`./bin/nameforge` via [DEVELOPING.md](DEVELOPING.md), then start it in a terminal. Both
stdin and stdout must be terminals. `nameforge tui` is the explicit entry point;
`nameforge tui --help` works without a terminal. Headless no-argument invocation
returns usage and exit 2; use `generate` for scripts.

The picker opens with **no categories selected**. All ten embedded given-name categories
work offline on first launch, without a writable home, network, Go installation,
or separate data. `--data-dir <path>` is accepted by `tui` as a reserved local-state
path, but no local packs, preferences, or favorites are loaded or saved there.

## Walkthrough

1. Press `/`, type `french`, press Esc, then Space to select French. Repeat for
   Italian. Filtering hides other rows but preserves their selections. `a`
   selects **all** categories, including hidden ones; `c` clears all selections.
2. Tab cycles categories → search → settings → results. Shift-Tab reverses.
   In settings, up/down selects a field. Space or left/right cycles mode, gender,
   and existing-name allowance. Type numbers into the other fields; Ctrl-U clears
   before the cursor, Ctrl-A/E moves to start/end. Length fields left blank are
   automatic; an empty seed requests a random seed.
   **Name type** cycles given / surname / full. Full names pair the same category
   (or matching blends), given + space + surname. Appended surname order/length/
   existing fields independently control the surname in full mode (defaults:
   order 2, automatic lengths, novel). Gender applies only to given names.
   Surname-only mode uses shared order/length/existing controls. Picker counts
   switch to surname counts in surname mode; full mode shows both counts.
   Greek/Arabic surname gaps are marked unavailable, not silently excluded.
3. Enter generates using current settings. Results appear asynchronously, with
   the actual seed, completion state, category attribution, and batch size.
   Mode selection uses the same [category/blend semantics as the CLI](CLI.md#category-gender-and-script-semantics).
4. Up/down or j/k selects a result; Space toggles it as a session favorite.
   `r` generates with a fresh random seed using current settings. An entered seed
   remains in settings, so Enter can replay it; each batch displays its actual seed.
5. Press `e` to export. Tab moves through target, format, and destination path;
   Space toggles current batch/session favorites and JSON/text. Enter writes the
   file. Changing format automatically updates the filename extension to `.txt`
   or `.json`, preserving its directory and basename. Existing destinations require
   `y` confirmation; `n` declines and allows
   editing the destination. Esc dismisses the dialog. Parent directories must
   already exist. Write failures keep the batch and favorites available for retry.
6. Press `?` for scrollable controls, full current status, effective length bounds,
   corpus/algorithm identity, source labels and coverage limitations. Esc dismisses
   help. Press `l` outside text entry or within help for complete GPL, dependency
   and corpus notices (up/down scrolls; `l` switches back to help). The startup
   heading displays copyright, license and no-warranty information.
   `q` quits outside text entry. Ctrl-C quits globally and restores the
   terminal, including during generation or export.

Search, numeric/seed fields and export paths consume ordinary characters such as
`q`, `j`, `r`, `e`, and `?` as text. Arrow keys still move between settings fields;
j/k only navigate outside text entry. Clipboard integration is not enabled.

## Seeds, errors and cancellation

The TUI shares `generator.NormalizeRequest` and the exact headless generation
engine. Category IDs are sorted and deduplicated, so picker order does not alter
seeded output. Batch metadata describes the settings used for that batch, even
if you subsequently edit settings or category selection.

Replay a displayed seed using the same category IDs, mode, gender, count, order,
length overrides, name type, surname settings and existing-name policy in
[the CLI](CLI.md). For defaults:

```sh
nameforge generate --category french --category italian --mode category --seed 42 --format json
nameforge generate --category french --category italian --mode blend --seed 42 --format json
nameforge generate --name-type full --category french --category italian --seed 42 --format json
```

Use the seed actually displayed for your batch in place of `42`. The current-batch
JSON export has the same v1 format as CLI JSON and retains every reproduction field.
Selecting all in the picker records an explicit list of IDs; `--all-categories`
produces the same ordered names, but records that selection shortcut in CLI options.

Generation runs in cancellable commands, never in rendering or blocking keyboard
updates. Esc cancels an operation; newer requests supersede older work. Late
completions cannot replace newer results. Previous results and favorites remain
available on cancellation or a failure with no accepted names. Bounded exhaustion
with accepted names displays an **INCOMPLETE** batch and recovery suggestions;
exporting that batch preserves `complete: false` and rejection/attempt metadata.

Gender/script filters use the same [CLI semantics](CLI.md#category-gender-and-script-semantics):
use `any` for Greek or all-category requests; current unisex pools are empty.
Missing filtered data is a recoverable error, never an English fallback. All output
remains Latin-only. See [source scope and notices](ROMANIZED.md) for Greek/Arabic.

## Export formats and session lifetime

- **Current batch JSON:** one `schema_version: 1` result, identical to the CLI's
  format, including all accepted names and full replay metadata.
- **Favorites JSON:** one `schema_version: 1` object with `batches`. Each batch has
  `result` (the original versioned result, including its full original batch) and
  `selected_names` (only its favorited name/attribution objects). This preserves
  original seeds, options, completeness and counters across different generations.
  Batches are grouped in first-favorite order; selections retain favorite order.
  Surname/full-name favorites keep name type, component spellings/attribution,
  both corpus hashes and component settings with the original batch.
- **Text:** one selected name per line, with no ANSI or metadata. Current-batch
  names retain generation order; favorites use the batch/selection ordering above.

Writes use a temporary sibling and publish only complete content. Without explicit
overwrite confirmation, even a concurrently created destination is protected.
An export already published before cancellation may remain on disk. Favorites
are session-only and disappear on quit; export them to retain them. No hidden
local state is created.

## Terminal layouts and checks

The focused panel uses the available space and scrolls to keep its selected row
visible. Category/settings summaries, the active batch seed, status and hints stay
visible. Below 20 columns or 8 rows, a resize prompt replaces the normal layout.
`nameforge tui --no-color`, a declared `NO_COLOR` environment variable (including
an empty value), or `TERM=dumb` disables text styling. The TUI still uses terminal
cursor/alternate-screen controls; headless output has no terminal escapes.

```sh
mise exec -- go test ./internal/tui ./internal/store ./internal/cli
mise exec -- go test -count=1 -v ./cmd/nameforge -run TestBinaryTUI
```

The macOS/Linux pseudo-terminal test exercises French +
Italian, romanized Greek + Arabic, and all categories in both modes; settings,
JSON/text exports, displayed-seed CLI replay, cross-batch favorites, source/script
labels, gender-error recovery, narrow/no-color layout, and Ctrl-C/SIGINT during
active work, including terminal modes and alternate-screen restoration.
[RELEASE.md](RELEASE.md#ci-release-workflow-and-target-claims) describes isolated
home/PATH, native runners, network-denial scope, artifact tests and owner-reported
human acceptance; cross-building alone does not claim native verification.
