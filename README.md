# Nameforge

A planned local, Go-based terminal UI for generating names with character-level
Markov chains trained on real, externally sourced name lists.

**Status: planning only.** The application and data importer are not implemented.
Nameforge is the working project/binary name.

## Development handoff

- [Implementation plan](docs/PLAN.md): architecture, behavior, ordered work items,
  acceptance criteria, and suggested atomic Conventional Commits.
- [Data sources](docs/DATA.md): Behind the Name import strategy and provenance.
- [Agent instructions](AGENTS.md): workflow and non-negotiable constraints.

## Toolchain

Use [mise](https://mise.jdx.dev/) for tools and exact versions. Go is pinned in
`mise.toml`; after reviewing this repository's configuration:

```sh
mise trust
mise install
mise exec -- go version
```

Build, test, run, format, and verification tasks will be added in milestone M1.
Go libraries will be pinned in `go.mod` / `go.sum`, not installed globally.

## Data and operation

The primary source is [Behind the Name](https://www.behindthename.com/names/list).
The owner reports having acquired email permission for personal use. Import real
name lists into local storage, then generate entirely offline. Agents must never
invent training names or replace missing source data with AI-authored lists.

Distributable binaries and personal data are separate: the default release will
not embed the personal corpus. A fresh installation guides the user to import
data; an installed corpus requires no network connection for generation.
