# Implementation instructions

Read `docs/PLAN.md` and `docs/DATA.md` before implementation. Implement the next
ready milestone and its acceptance criteria; record completed work and checks in
the plan before handing off. This initial repository is a planning deliverable.

## Constraints

- Go application, local TUI, character-level Markov generation for TTRPG use.
- Embed licensed multilingual data so the binary works offline on first launch.
  Support selecting one or more categories and explicit category/blend modes.
  Prioritize Mediterranean/Western Europe and Turkish. Do not relabel a generic
  Arabic list as North African or substitute English fallback for missing data.
- Real externally sourced training data only. Do not author name corpora or seed
  lists, including demonstration lists. Small algorithm fixtures may use clearly
  non-name token sequences; name-bearing fixtures must have source provenance.
- Faker's static locale name arrays are the planned bundled source (MIT).
  Extract source arrays, not generated Faker outputs; preserve full notices.
  Never use implicit English fallback or equate unspecified gender with unisex.
- Behind the Name is an optional later personal source and UX reference; the
  user has permission for personal use. No repeat permission prompt is required.
- Use mise for all developer tool versions and command tasks. Pin exact versions;
  run Go commands through `mise exec --` or mise tasks. Pin dependencies in Go
  module files. Do not introduce another version manager or depend on global Go.
- Keep generation independent of the terminal, filesystem, and network.
- Commit the reproducibly derived redistributable corpus, source lock and notices.
  Keep personal corpora, fetched HTML, generated names, and correspondence out of
  commits. Embed only the explicit redistributable asset directory.
- Maintain Unicode and deterministic seeded generation as specified in the plan.

## Code Review Rules

- Report only actionable defects introduced by the pull request. Prioritize
  correctness, data provenance and licensing, security, and runtime/build
  regressions. Give the concrete impact and location; skip style preferences,
  speculative risks, and pre-existing issues.
- Treat `docs/PLAN.md` and `docs/DATA.md` as behavioral contracts. In particular,
  check seeded determinism, Unicode rune/NFC handling, category and gender
  semantics, explicit errors for missing data, and the absence of English fallback.
- For corpus changes, verify training names come from externally sourced static
  arrays and retain traceable source references, pinned revisions/checksums, and
  complete license notices. Flag authored or generated training names, inaccurate
  cultural labels, and treating unspecified gender as unisex.
- Preserve the offline architecture: generation stays independent of terminal,
  filesystem, and network I/O. Built-in corpora must load offline from embedded
  redistributable assets without writable local state. Explicitly selected local
  packs may load through documented storage boundaries; runtime never fetches
  training data over the network.

## Workflow

- Inspect status and existing work before changing files. Do not overwrite another
  agent's changes. Follow milestone dependencies and avoid speculative features.
- Use atomic Conventional Commits: `feat(scope):`, `fix(scope):`,
  `test(scope):`, `docs(scope):`, `chore(scope):`, `ci(scope):`.
- Keep one coherent change per commit; include tests and docs that belong to it.
  Do not combine unrelated refactors, data refreshes, and UI changes.
- Run relevant acceptance checks before committing. Before commit, inspect
  `git status`, `git diff`, and recent history; stage intended files explicitly.
- Never add AI co-author trailers. Do not push or create a remote unless asked.
- Handoff with completed milestone IDs, checks actually run, limitations, and the
  next ready milestone. Do not mark a milestone complete based on scaffolding.
