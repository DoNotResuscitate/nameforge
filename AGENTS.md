# Implementation instructions

Read `docs/PLAN.md` and `docs/DATA.md` before implementation. Implement the next
ready milestone and its acceptance criteria; record completed work and checks in
the plan before handing off. This initial repository is a planning deliverable.

## Constraints

- Go application, local TUI, character-level Markov generation.
- Real externally sourced training data only. Do not author name corpora or seed
  lists, including demonstration lists. Small algorithm fixtures may use clearly
  non-name token sequences; name-bearing fixtures must have source provenance.
- Behind the Name is the primary source; the user has email permission for
  personal use. No additional permission prompt is required for personal import.
- Use mise for all developer tool versions and command tasks. Pin exact versions;
  run Go commands through `mise exec --` or mise tasks. Pin dependencies in Go
  module files. Do not introduce another version manager or depend on global Go.
- Keep generation independent of the terminal, filesystem, and network.
- Keep local corpora, fetched HTML, generated names, and permission correspondence
  out of commits. Binary distributions do not embed personal datasets or models.
- Maintain Unicode and deterministic seeded generation as specified in the plan.

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
