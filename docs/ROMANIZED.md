# Romanized Greek/Arabic source review

Reviewed on 2026-10-01 for M5a. These pinned sources remain the current runtime
packs; this document preserves selection evidence, not a pending research task.
See [coverage](COVERAGE.md) for the full bundle and [data contract](DATA.md) for schema.

Greek and Arabic use real source-supplied Latin spellings, with pinned revisions,
raw SHA-256 checksums, complete notices and per-record references. No generated
Faker outputs, authored seed lists or project transliterations enter training.
The original eight other Faker packs are unchanged.

The intended setting is **early-modern fantasy, approximately the mid-1600s,
forty years after a cataclysm**. The user accepts mixed ancient and modern Greek
material. This contextualizes the pack's breadth; it does not make these lists a
historically representative 1600s population sample. Generated names are stylistic
TTRPG candidates, not guaranteed linguistically or historically valid names.

## Greek — Wikipedia static Latin/Greek pairs

- Article: [Greek name, revision 1377658475](https://en.wikipedia.org/w/index.php?title=Greek_name&oldid=1377658475).
- [Pinned raw wikitext](https://en.wikipedia.org/w/index.php?title=Greek_name&oldid=1377658475&action=raw).
- Raw SHA-256: `a5e6a986d33b1e63e2a80004a005a641779b9f865cb5033ff9af9bc1d56d44d2`.
- Scope: the **Ancient names** and **Biblical and Christian names** numbered lists.
  They include historical, mythological, Christian and modern/secular examples.
  The headings are source locations, not a claim that every row fits that era.
  The separate mixed-name construction and diminutive sections are not imported.
- Spellings: literal visible link labels, or the literal title when no display
  label is supplied, with name/disambiguation suffixes removed. Supplied slash/or
  alternatives are separate spellings. Links/templates are never evaluated;
  the sole interlanguage-link row is handled as a reviewed literal construct.
- Convention: mixed conventional classical Latin forms and modern Greek renderings;
  no claim of uniform ISO 843/ELOT 743 spelling. Source accents are preserved.
- Gender: **unspecified for every entry**. The source lists lack row-level gender
  evidence; endings and external Faker data do not supply it.
- Coverage: 280 ancient rows + 242 biblical/Christian rows; four additional supplied
  alternatives yield 526 spelling occurrences. After 31 rejections and nine merges:
  **486 distinct spellings**, all Latin letters, **2–13 NFC runes**.
- Source quality: a community-edited examples list with citation/copyediting
  maintenance notices. It is not an authoritative or frequency-ranked register.

### Reviewed Greek exclusions

Indices are zero-based within the named numbered list. Source spelling is never
repaired. Every rejected occurrence retains the section/row/variant and original
Greek association in `data/quality.json`.

| Section | Indices | Reason |
| --- | --- | --- |
| Ancient names | 1, 8, 9, 20 | Malformed Latin/Greek pairings |
| Ancient names | 108, 129 | English equivalents rather than the supplied Greek form |
| Ancient names | 139 | Mismatched Greek association |
| Ancient names | 152 | Epic title, not a given name |
| Biblical and Christian names | 2, 15, 22, 28, 77, 97, 126, 127, 128, 144, 145, 146, 150, 178, 225, 239 | Foreign equivalents rather than Greek spellings |
| Biblical and Christian names | 49, 147 | Mismatched associations |
| Biblical and Christian names | 207 | Linked source explicitly identifies a surname |

Four further source labels contain Greek letters inside otherwise Latin text:
ancient row 59 and biblical/Christian rows 8, 160 and 233. They fail Latin spelling
validation instead of being silently repaired. Conventional historical spellings
are retained under the agreed mixed-era scope. The extractor selects the visible
labels **Dorothea** and **Loukia**, not the English link targets Dorothy and Lucy.

### Greek licensing

Wikipedia text is CC BY-SA 4.0. The Greek-derived records and adaptations retain
that license. `licenses/WIKIMEDIA-NOTICE` attributes the article and contributors,
links the revision and contributor history, describes changes and identifies the
licensed material. `licenses/CC-BY-SA-4.0` preserves the complete legal text.
These notices are embedded and exposed by `nameforge licenses`; other packs keep
their own source licenses and application code retains the repository GPLv3.

## Arabic — revision-pinned Wikidata statements

- Source: Wikidata structured data, [released under CC0](https://www.wikidata.org/wiki/Wikidata:Licensing).
- Inventory: 119 immutable entity snapshots, each locked with QID, revision,
  exact `Special:EntityData/QID.json?revision=REVISION` URL and raw SHA-256.
  Fetching validates that the returned entity ID and `lastrevid` match the lock.
- Selection: direct `P31` male given name (`Q12308941`), female given name
  (`Q11879590`) or given name (`Q202444`); `P407` Arabic (`Q13955`); Arabic-tagged
  `P1705` association containing Arabic-script letters; source-supplied `P1705`
  Latin text tagged `mul`. Deprecated statements are not used. English labels
  and aliases never supply training spellings.
- `mul` means multiple languages, **not automatically Arabic romanization**.
  Pairings therefore received a further review, with mismatched and specifically
  Persian/Turkish/South/Southeast Asian forms held out as listed below. Shared
  Latin forms remain usable where the Arabic association supports them.
- Convention: heterogeneous supplied Latin renderings, including French-oriented
  forms and diacritics. No consistent transliteration standard is documented.
  No regional scope or North African provenance is inferred from a spelling.
- Gender: map only the direct given-name class statements. Generic supplies no
  gender; if the same entity also explicitly has a gendered class, retain that
  evidence. No source gender transfers from an unrelated spelling/list.
- Provenance: original `claims.P1705` index and statement ID; native Arabic
  associations; supporting native-label, Arabic-language and gender-class statement
  IDs. Native association strings remain untouched, including source bidi format
  characters. They are provenance only, never Latin training or generated output.
- Coverage: **137 class/spelling occurrences**, 25 rejected, 112 accepted and
  three merged: **109 distinct spellings**, 22 feminine and 87 masculine,
  **3–13 NFC runes**. No accepted dual-gender spellings remain after review.
- Bibliographic limitation: selected Latin statements are mostly unreferenced.
  Two accepted occurrences have external references (an athletics list and an INE
  name spreadsheet), neither specifying a romanization standard. Revision and
  statement traceability is complete; bibliographic support and conventions are
  sparse. This is a small community-curated Arabic-name pack, not a population study.

### Discovery and pinning

Research used this live query to find candidates; it is **not** run by fetch,
rebuild, tests or runtime:

```sparql
SELECT DISTINCT ?item ?class ?native ?latin WHERE {
  VALUES ?class { wd:Q12308941 wd:Q11879590 wd:Q202444 }
  ?item wdt:P31 ?class ; wdt:P407 wd:Q13955 ; wdt:P1705 ?native, ?latin .
  FILTER(LANG(?native)="ar")
  FILTER(LANG(?latin)="mul")
  FILTER(REGEX(STR(?latin), "^[A-Za-zÀ-ÖØ-öø-ÿĀ-ž .-]+$"))
} ORDER BY ?item ?class ?latin ?native
```

The live discovery returned 119 entities and 129 exact spellings; pinned entity
JSON is authoritative and includes additional statements absent from that query's
regex. The frozen roster supplies only QIDs/revisions to maintenance `pin`; all
spellings and classifications are re-extracted from those raw snapshots. A future
refresh must review a new inventory explicitly. No live query or English-label
fallback is used to fill gaps.

### Reviewed Arabic exclusions

Whole-entity holdouts: `Q639748`, `Q21946841`, `Q63919383`, `Q113517732`,
`Q479503` (ambiguous multilingual alternatives, held out rather than corrected);
`Q21069195`, `Q28790469`, `Q139882721` (Persian-script associations);
`Q59051920` (Persian/Urdu reading without verified Arabic convention).

Statement-only holdouts preserve other suitable **source-provided** alternatives:

| Entity | Statement suffix | Reason |
| --- | --- | --- |
| Q7380459 | 1D5B62A5-803F-4554-A096-58854DB9F567 | Persian rendering |
| Q107260989 | 21408579-6109-4DC0-AA90-8C038B77541A | Persian rendering |
| Q21081645 | 13FA3431-7F19-4925-BB86-BF1B15E5C563 | Persian/Uzbek-specific association |
| Q3194364 | 98B01DB4-318D-44D2-B0CD-4BB1B4C13C97 | South Asian rendering |
| Q3607213 | E570AE9E-5467-4155-8244-4A12E489AEE2 | Southeast Asian rendering |
| Q4164677 | 53742042-75E4-48DD-A6AA-B4B6DC4457AC; FE34CE0C-A233-49B3-BA49-CBF7426FE879 | Turkish k-forms; supplied q-form retained |

`Q118151012` also fails automatically: its Arabic-tagged native field contains
Latin text rather than Arabic-script letters. No spelling is authored to replace
a holdout. Exact rejected references and reasons are in the quality report.

### Arabic licensing

`licenses/CC0-1.0` contains the complete CC0 legal text. The Wikimedia notice
identifies Wikidata, its licensing policy, extraction changes and per-record
attribution scheme. All notices are embedded, checksum-verified and exposed offline.

## Research candidates not selected

Luna research checked Python Faker, FFaker, Bogus and other Faker implementations;
the relevant Greek/Arabic arrays were native-script, absent or unsuitable. The
Algeria provider additionally cited third-party pages without a clear data reuse
grant. Wikidata Greek English labels were insufficient as romanization evidence;
paired Latin `P1705` Greek candidates were small and included misleading foreign
forms. Wikipedia's broad Arabic given-name page included many unrelated foreign
names, while its theophoric page had much narrower scope. Those lists were not
substituted for a broad Arabic pack. Behind-the-Name-derived mixed-provenance
repositories remain unselected as documented in [DATA.md](DATA.md).
