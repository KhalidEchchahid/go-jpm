# CLI Command Conventions

> Last updated: 2025-10-09

To keep the JPM CLI predictable, all read-only commands follow a consistent naming convention. The goal is to make it easy to guess the right subcommand without checking help output.

## Naming Guidelines

- **Use `ls` for listing resources.**
  - `jpm module ls` lists modules in the current project.
  - `jpm deps ls` lists declared dependencies.
- **Reserve descriptive nouns for resource groups.**
  - `module`, `deps`, `init`, and future categories act as namespaces for related actions.
- **Prefer Unix-style verbs for read-only operations.**
  - `ls` → list summaries.
  - `tree` → show hierarchical views.
  - `info` → display detailed metadata for a single resource (future).
- **Write operations use explicit verbs.** (planned)
  - `add`, `remove`, `upgrade`, `init`, etc.

## Legacy Command Mapping

| Old Command           | New Command       | Notes |
|-----------------------|-------------------|-------|
| `jpm module find`     | `jpm module ls`   | Unified with other list commands. |
| `jpm deps show`       | `jpm deps ls`     | Matches Unix-style list naming. |

## Authoring New Commands

When designing additional CLI functionality:

1. Choose the resource group (`module`, `deps`, `init`, ...).
2. Apply the verb guidelines:
   - `ls` for lists.
   - `tree` for graphs.
   - `add`/`remove`/`upgrade` for mutations.
3. Document the command under the appropriate section in `README.md` and update this conventions file.
4. Update integration tests in `cmd/jpm/cli_integration_test.go` to cover the new behavior.

Adhering to this convention avoids a mix of synonyms (`find`, `show`, `list`) and keeps the CLI discoverable for new contributors and users alike.
