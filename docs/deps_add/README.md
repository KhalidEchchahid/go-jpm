# `jpm deps add` – Implementation Blueprint

> Last updated: 2025-10-12

This document captures the end-to-end plan for building a senior-quality `jpm deps add` feature. It defines user experience, CLI surface area, data sourcing, caching, and shell completion strategies so that the work can be executed predictably by any contributor.

---

## 1. Problem Statement & Goals

### 1.1 What We Are Solving

- Provide a safe, ergonomic way to add dependencies to Maven/Gradle projects without hand-editing build files.
- Eliminate guesswork when searching for coordinates and versions by surfacing authoritative suggestions.
- Support tab-completion (Bash, Zsh, Fish, PowerShell) so users can discover dependencies inline, similar to AUR helpers.

### 1.2 Success Criteria

- `jpm deps add` adds or updates a dependency entry while preserving existing formatting and comments.
- Command offers interactive search + non-interactive flags for CI usage.
- Autocomplete completes both artifact coordinates and available versions, powered by a cached index.
- Works offline with a previously hydrated cache; degrades gracefully if index is missing.
- Full test coverage (unit + integration) for parsing, cache hydration, and build-file patching.

---

## 2. User Experience Overview

```bash
jpm deps add [group:artifact[@version]] [flags]
```

### 2.1 Primary Flows

1. **Happy Path (interactive search)**
   - User runs `jpm deps add` with no args.
   - CLI prompts for search term, shows ranked results with metadata (group, artifact, latest version, description).
   - User selects result, picks version (default = latest stable), optional scope classifier.
   - Tool writes dependency to build file, confirms action, offers to run verification (e.g., `mvn -q dependency:tree`).

2. **Direct Coordinate Input**
   - `jpm deps add com.squareup.okhttp3:okhttp`
   - CLI resolves latest stable version automatically (unless `--version` supplied).

3. **CI / Script Mode**
   - `jpm deps add com.squareup.okhttp3:okhttp --version 5.0.0 --scope test --build-tool maven --non-interactive`
   - Validates coordinates locally (from cache) or remotely (API) before mutation.

### 2.2 CLI Flags & Options

| Flag | Type | Description |
|------|------|-------------|
| `--build-tool`, `-b` | enum (`maven`, `gradle`) | Override build tool detection. |
| `--version`, `-v` | string | Pin to a specific version; accepts semver ranges (e.g., `^1.2`, `~3.1`). |
| `--latest` | bool | Force refresh remote metadata before selecting latest release. |
| `--scope`, `-s` | string | Maven scope / Gradle configuration (`compile`, `test`, `runtime`, custom). |
| `--classifier` | string | Optional classifier (`sources`, `javadoc`). |
| `--type` | string | Packaging type (`jar`, `pom`, etc.). |
| `--interactive` / `--no-interactive` | bool | Force interactive prompts on/off. Default = on if TTY. |
| `--refresh-index` | bool | Trigger cache refresh prior to completion or execution. |
| `--dry-run` | bool | Show planned changes without touching files. |
| `--output` | enum (`human`, `json`) | Choose output format for scripting. |
| `--backup` | bool | Force `.backup` file even if global setting disables it. |

> _Stretch:_ `--constraints` for Gradle version catalogs, `--project` to operate on sub-modules.

### 2.3 Interactive UX Enhancements

- Fuzzy search on group/artifact via inline fzf-style selector (optional dependency) or built-in ranking.
- Show popularity metrics (download counts) to guide choice.
- Display changelog summary when upgrading existing dependency.

---

## 3. Autocompletion Strategy

### 3.1 Goals

- Tab-completion suggests group:artifact combinations and versions.
- Works for Bash, Zsh, Fish, PowerShell using Cobra’s completion hooks.
- Fast enough to keep shell responsive (<100ms per completion).

### 3.2 Data Sources

1. **Primary:** Maven Central Search API (`https://search.maven.org/solrsearch/select`).
2. **Secondary:** Local `~/.m2/repository` index for offline modules.
3. **Gradle Portal:** REST API for plugins/configurations (Phase 2+).
4. **Optional:** Custom curated allowlists for internal artifacts.

### 3.3 Cache Hydration Pipeline

```text
+-------------------------------+
| jpm deps hydrate (new command)|
+---------------+---------------+
                |
                v
     +--------------------+
     | Remote Index Fetch |
     |  - Maven Central   |
     |  - Gradle Portal   |
     +--------------------+
                |
                v
     +--------------------+
     | Normalize & Rank   |
     |  - groupId         |
     |  - artifactId      |
     |  - latest version  |
     |  - release date    |
     |  - download stats  |
     +--------------------+
                |
                v
     +--------------------+
     | Persist Cache      |
     |  ~/.cache/jpm/deps |
     +--------------------+
```

- Cache stored as compressed JSON/SQLite (choose based on size; SQLite recommended for indexing).
- Each record holds: `groupId`, `artifactId`, `versions` (sorted), `latest`, `popularity`, `licenses`.
- Provide TTL (e.g., 24h) + command to refresh.

### 3.4 Completion Hook Design

- Leverage Cobra’s dynamic completion: implement `ValidArgsFunction` on `deps add`.
- Completion function steps:
   1. Ensure cache is present (hydrate lazily if missing and online).
   2. Parse current word; if contains `:` or `@`, split into coordinate + version portion.
   3. Query cache using prefix search (e.g., `SELECT ... WHERE groupArtifact LIKE 'com.squa%' LIMIT 50`).
   4. Output `group:artifact` suggestions annotated with the latest release. When the user types `group:artifact@` we surface version-specific completions filtered by the typed prefix.
- Provide fallback completion when cache missing: show helpful message to run `jpm deps hydrate`.

### 3.5 Shell Integration

- Update install docs: `jpm completion bash|zsh|fish|powershell`.
- Document requirement to re-run `jpm completion` after upgrades adding new completions.
- Add automated tests for completion output (`cobra.MarkFlagRequired` interplay).

---

## 4. Dependency Mutation Workflow

### 4.1 Build-Tool Abstractions

- Extend `ProjectInspector` with mutation helpers or introduce `ProjectEditor` interface.
- Implement Maven editor using DOM-aware library (e.g., `github.com/beevik/etree`) to preserve formatting.
- For Gradle (Groovy DSL initially):
  - Detect whether dependency block exists; insert while respecting indentation and quoting.
  - Provide guard rails for Kotlin DSL (phase 2).

### 4.2 Conflict Handling

- If dependency already present:
  - Offer upgrade path (prompt or `--replace`).
  - Support `--allow-duplicate` for multi-scope additions.
- Validate scopes vs build tool (e.g., `testImplementation` vs `testCompile`).

### 4.3 Safety Nets

- Auto-create `.backup` file before mutation (toggle via config).
- Dry-run output shows unified diff (`git diff --no-index` style) to build trust.
- Rollback command (`jpm deps undo`) for future work.

---

## 5. Architecture & Components

| Component | Responsibility |
|-----------|----------------|
| `internal/adapters/maven/editor.go` | Insert/update `<dependency>` nodes, maintain ordering, ensure newline at EOF. |
| `internal/adapters/gradle/editor.go` | Modify `build.gradle` or `build.gradle.kts`; handle block detection. |
| `internal/catalog/cache` | Manage hydrated dependency catalog (SQLite/JSON + indexing). |
| `internal/catalog/fetch` | Remote API clients (Maven Central, Gradle Portal). |
| `internal/catalog/search` | Fuzzy + prefix search algorithms for completions and interactive UI. |
| `cmd/jpm/deps_add.go` | Cobra command definition & wiring. |
| `cmd/jpm/deps_hydrate.go` | Optional command to refresh cache on demand. |
| `cmd/jpm/style.go` | Extend formatting utilities for diffs, prompts, error styling. |

---

## 6. Implementation Phases

1. **Phase 0 – Foundations**
   - Define data models for dependency catalog.
   - Add configuration + cache directory helpers.
   - Ship `jpm deps hydrate` (fetch + cache).

2. **Phase 1 – CLI Skeleton**
   - Implement `deps add` Cobra command with flags.
   - Wire completion hook to cache (basic prefix search).
   - Provide stubbed mutation (dry-run only) to validate UX.

3. **Phase 2 – Build File Editing (Maven)**
   - Implement Maven dependency insertion with formatting preservation.
   - Add integration tests using fixtures in `java-legacy/JPM`.

4. **Phase 3 – Advanced UX**
   - Interactive fuzzy finder (optional dependency or built-in).
   - Popularity metrics + warnings on pre-release versions.
   - JSON output for automation.

5. **Phase 4 – Gradle Support**
   - Implement Groovy DSL editing.
   - Add Kotlin DSL detection fallback (warn if unsupported).
   - Expand cache coverage with Gradle metadata (modules, catalogs).

6. **Phase 5 – Polish & Observability**
   - Telemetry hooks (if desired) to track command usage.
   - Performance optimization (index compaction, incremental refresh).
   - CLI docs & tutorials.

---

## 7. Edge Cases & Risk Mitigation

- **Monorepos:** Detect module-specific build files; allow `--module` flag.
- **Existing Version Constraints:** Handle property-based versions (`${okhttp.version}`); prompt to update property or create one.
- **Profiles & Conditionals:** Respect Maven profiles; allow `--profile` to target.
- **Gradle Version Catalogs:** If project uses `libs.versions.toml`, integrate with catalog (future phase).
- **Offline Mode:** If cache stale and offline, warn but allow manual entry.
- **Rate Limiting:** Batch API queries, respect Maven Central limits (~30 req/s). Implement exponential backoff + user messaging.

---

## 8. Testing Strategy

- **Unit Tests:**
  - Cache fetchers (mock HTTP).
  - Search ranking, fuzzy matching.
  - Build-file editors for various edge cases.
- **Integration Tests:**
  - Execute `deps add` against `java-legacy/JPM` fixtures (Maven multi-module).
  - Shell completion snapshot tests (Bash, Zsh).
- **Smoke Tests:**
  - CLI dry-run with no cache (ensures helpful guidance).
  - Cache hydration command with simulated API responses.

---

## 9. Developer Tooling & Ops

- Introduce `make deps-add-dev` target to:
  - Run cache hydration against mock server.
  - Execute integration tests.
  - Generate completions locally.
- CI pipeline steps:
  - Lint (gofmt, golangci-lint).
  - Unit + integration tests.
  - Regenerate cache fixture (small sample) used in tests.

---

## 10. Follow-up Tasks Checklist

- [ ] Finalize storage backend choice (SQLite vs compressed JSON).
- [ ] Draft API client with retry/backoff + user agent identification.
- [ ] Define cache schema & migration strategy.
- [ ] Implement `deps hydrate` command + config option for auto-refresh.
- [ ] Extend `ProjectInspector` or introduce `ProjectEditor` interface.
- [ ] Create Maven editor with formatting preservation + tests.
- [ ] Stub Gradle editor, log "not yet supported" with actionable guidance.
- [ ] Implement Cobra completion hook with cache-backed suggestions.
- [ ] Add interactive search UI (optional dependency flag `--ui`).
- [ ] Update documentation (`README.md`, `CLI_COMMAND_CONVENTIONS.md`) once feature ships.

---

## 11. Open Questions

- Should we ship a curated “starter catalog” for instant suggestions without hydration?
- Do we need API keys / rate limiting strategies for enterprise users (Artifactory, Nexus)?
- How do we surface release notes or changelog snippets during upgrades?
- Should `deps add` auto-run a verification task (configurable) after mutation?

---

## 12. Next Steps

- Align with stakeholders on phases & timelines.
- Create GitHub issues per phase referencing this blueprint.
- Begin implementation with cache hydration (Phase 0).
