# JPM Init Prototype (Interactive, Prompt-First UX)

Last updated: 2025-10-29
Owner: Core CLI
Status: Draft prototype spec

---

## Goals
- Deliver an npx create-next-app style, zero-flags, interactive `jpm init` that sets up a modern Java project with a flat `src/`.
- Hide build-tool complexity behind a `.jpm` dotfolder with generated wiring (symbolic links, build files).
- Use a human-readable manifest (YAML) instead of JSON to describe project metadata and dependencies.
- Completely abstract Maven/Gradle from users: build/run/test/deps are JPM-only commands; dotfolder tools are never exposed.
- jpm.yaml is the single source of truth for dependency declarations and versions; tool files are derived.

## UX Principles
- Prompt-first, minimal flags. Flags exist but are not required; defaults and prompts guide the experience.
- Friendly, safe, and reversible. Validate decisions, preview final layout, and only write on confirmation.
- Platform-aware. Handle JDK detection and installation guidance across macOS/Linux/Windows.
- Progressive disclosure. Start simple; expand prompts only when relevant.

---

## First-Run Flow (Happy Path)
1) User runs `jpm init` in an empty directory (or provides a name to create a new one).
2) JPM detects Java and offers to install if missing (sdkman on macOS/Linux, winget/choco on Windows, or manual path entry).
3) User picks a template: Plain Java (prototype), future: Spring, Micronaut, Quarkus.
4) User confirms metadata: project name, groupId, artifactId, version, Java version.
5) JPM scaffolds:
   - `src/` (flat, any structure allowed by user later)
   - `.jpm/maven/` (prototype) with `pom.xml`
   - Symlink `.jpm/maven/src/main/java -> ./src` (Windows: junction fallback)
   - `jpm.yaml` manifest
   - Optional minimal `Main.java` if `src/` empty
6) Final summary + next steps (how to build/run).

---

## Example Transcript (Prototype)
```
$ jpm init

Welcome to JPM — Java projects without the yak shaving!

• Where should we create your project?
  1) Here (.)
  2) In a new folder
> 2

• Project folder name:
> my-app

• Java not found on PATH.
  Would you like JPM to help install JDK 21?
  1) Yes, using SDKMAN (macOS/Linux)
  2) Yes, using Homebrew (macOS)
  3) I’ll install manually and re-run
  4) I have a JDK path to use now
> 1
(… show short instructions or run installer if user consents …)

• Choose a template:
  1) Plain Java (JPM-managed prototype)
  2) Spring Boot (coming soon)
  3) Gradle variant (coming soon)
> 1

• Project metadata (press Enter to accept defaults)
  Name: my-app
  Group ID: app
  Artifact ID: my-app
  Version: 0.1.0-SNAPSHOT
  Java version: 21

Preview
  - Create: my-app/
  - Create: my-app/src/
  - Create: my-app/.jpm/maven/pom.xml
  - Link:   .jpm/maven/src/main/java -> ./src
  - Create: my-app/jpm.yaml
  - Create: my-app/src/Main.java (hello world)

Proceed? (Y/n)
> Y

✔ Project initialized at ./my-app
Next steps:
  cd my-app
  jpm build
  jpm run
```

---

## Project Structure (Prototype)
- src/
  - User-owned Java source (any structure or packages they prefer)
- .jpm/
  - maven/
    - pom.xml (minimal, generated)
    - src/main/java -> symlink to ../../src (Windows: directory junction)
- jpm.yaml (project manifest; authoritative source for JPM features)

---

## Manifest (YAML) — jpm.yaml (Prototype)
```yaml
# JPM project manifest (prototype)
name: my-app
build_tool: maven
engine: maven  # internal; JPM abstracts this
java:
  version: "21"
project:
  group_id: app
  artifact_id: my-app
  version: 0.1.0-SNAPSHOT
dependencies:
  - group: com.google.guava
    artifact: guava
    version: "33.0.0-jre"
  - gav: "org.slf4j:slf4j-api:2.0.16"
```

Notes
- YAML chosen for readability, comments, and trailing commas not needed.
- JPM is the source of truth; `.jpm/maven/pom.xml` is generated/derived where possible.
- jpm.yaml owns dependency declarations entirely; users never edit tool files.

### Manifest: Dependency schema (v0)
- dependencies: list of entries; each entry is one of:
  - { group, artifact, version, scope?, classifier? }
  - { gav: "group:artifact:version" }
- Allowed scopes (phase 1): compile, test; defaults to compile.
- Validation: group/artifact non-empty; version non-empty; strings only.
- Future: support BOMs, exclusions, platforms.

---

## JPM Engine (Internal Abstraction)
Goal
- Keep UX 100% JPM (no mvn/gradle surfaced) while leveraging a hidden engine to build/run/test and manage deps.

Engines (phased)
- maven: Phase 1. Use a pinned Maven runtime inside .jpm to compile/package. Users never see it.
- native: Phase 2+. Replace mvn with a native JPM pipeline using javac/jar and a resolver. Same UX.
- gradle: Future. Only if templates require Gradle-specific behaviors; still hidden.

How commands map
- jpm build → engine.Build(plan)  // resolve deps, compile with javac, package JAR
- jpm run   → engine.Run(entry)   // runs selected main class from built artifact
- jpm test  → engine.Test()       // runs tests (future)
- jpm deps  → engine.Deps()       // add/show/tree using manifest as source of truth

Isolation & reproducibility
- Version pinning: engine and tool versions recorded in jpm.yaml (internal fields) and .jpm/engine.lock.
- Sandboxed workdir: builds run in .jpm/work with normalized env vars.
- Per-project caches in .jpm/cache with sensible defaults; support offline mode.

Output & errors
- Normalize output: show concise JPM progress; hide engine logs by default; `--verbose` reveals logs.
- Error translation: map engine-specific errors to friendly JPM messages with actionable hints.
- Logs stored at .jpm/logs/*.log for debugging.

Security & safety
- Sanitize all engine inputs; forbid arbitrary script execution in default templates.
- Warn when third-party build scripts/plugins are detected; require explicit consent.

Windows & symlinks
- Prefer symlink; fallback to junction/copy with clear messaging; engine resolves real paths transparently.

Roadmap switch
- Start with engine: maven (pinned, vendored or auto-fetched into .jpm).
- Transition to engine: native without changing user-facing commands.


## Build Pipeline (ASCII)
```
JPM CLI (init / build / run / deps)
        |
        v
  Command Router
        |
        v
  Manifest Loader (jpm.yaml)  ->  Plan Builder
        |
        v
  Engine Selector [maven | native | gradle]
        |
        +---------------------------+
        |                           |
        v                           v
 Engine: maven (Phase 1)       Engine: native (Phase 2+)
   - pinned mvn in .jpm/tools     - resolve deps (manifest)
   - runs in .jpm/work            - fetch/cache -> .jpm/cache
   - caches -> .jpm/cache         - javac -> classes -> jar
   - logs -> .jpm/logs            - logs -> .jpm/logs
        \                           /
         \                         /
          +---- Artifacts -----> .jpm/out/*.jar (and/or target/*.jar)

Outputs:
- Artifacts: jars in .jpm/out (prototype may also write Maven target/)
- Logs: .jpm/logs/*.log
- Exit codes: propagated to CLI, errors translated to friendly messages
```

## Error Handling & Edge Cases
- Non-empty directory: show contents summary and require explicit confirmation.
- Java missing: present install choices and a manual PATH entry option; allow skipping with clear next steps.
- Symlink creation failure (Windows or restricted FS): fallback to directory junction/copy; explain implications.
- Offline: skip dependency lookups, continue with local scaffold.
- Re-run safety: if files exist, show diff/overwrite prompts (or `--force` if user insists, but keep prompts primary UX).

---

## Accessibility & Output
- No color-only cues; include text symbols and labels.
- Short, scannable steps; numbered choices; sensible defaults pre-selected.
- Pasteable commands in guidance blocks.

---

## Telemetry (Optional, Off by Default)
- Prompt: "Help improve JPM by sending anonymous usage data? [Y/n]"
- If enabled, record: command, timings, platform (no PII, no paths).

---

## Implementation Plan (Prototype Scope)
Phase 0 — Foundations
- Add `init` Cobra command that runs fully interactive (avoid flag-first UX).
- Add prompt engine (Go: survey or custom minimal prompts) with keyboard navigation and validation.
- Add cross-platform Java detection (exec.LookPath + JAVA_HOME; parse `java -version`).
- Define manifest schema v0 (including dependencies) and implement YAML parser/serializer and validators.
- Add engine scaffold with interface (Build/Run/Test/Deps) and version pinning in jpm.yaml. See internal/core/engine.go for the initial API.

Phase 1 — Scaffold
- Create target directory (or current), warn if not empty.
- Create `src/` and write `Main.java` only if empty.
- Prepare `.jpm/` structure and symlink `src/main/java -> src`; render tool files (e.g., pom.xml) during `jpm build` from manifest.
- Write `jpm.yaml` from prompt answers (include empty `dependencies: []`).
- Implement `jpm deps show` (reads manifest) and `jpm deps add` (writes manifest) with an interactive prompt (manual version entry for now).
- Implement `jpm build` and `jpm run` that use the engine internally (no mvn/gradle shown).
- Print summary + next steps.

Phase 2 — Install Assist (opt-in)
- Offer SDKMAN bootstrap on macOS/Linux (script with explicit consent).
- Offer Homebrew alternative on macOS; winget/choco hints on Windows.
- If user provides a local JDK path, validate by invoking `bin/java -version`.

Phase 3 — Niceties
- `.gitignore` (target/, .idea/, .classpath, .settings, .project, .jpm/* build artifacts).
- Optional README.md with quickstart.
- Windows symlink fallback to junctions; auto-detect permissions.

Out of Scope (Future Sprints)
- Gradle template parity; framework templates (Spring, Micronaut, Quarkus).
- Dependency wizard (searching Maven Central) — for `jpm deps add`.

---

## Detailed Task Breakdown
1) Prompt Engine
   - Implement select, input, confirm primitives
   - Validation: non-empty names; Java version set {17,21,23}; safe artifactId
   - Windows input quirks handling

2) Environment & Detection
   - `java` on PATH, JAVA_HOME checks
   - Parse `java -version` to extract major
   - Installer helpers (sdkman hints, brew/winget/choco commands)

3) Scaffolding
   - Directory creation helpers with dry-run preview
   - Symlink/junction abstraction with capability detection
   - Cross-platform file permissions, UTF-8 writer

4) Generators
   - Minimal `pom.xml` template with compiler properties
   - `jpm.yaml` generator (YAML)
   - Hello World `Main.java`

5) Safety & Idempotence
   - Non-empty dir dialog (list up to N files; "Show more" option)
   - Overwrite policy: skip existing, or prompt per file, or write `*.new`
   - Rollback on error: if scaffold partially written, remove created items

6) UX Polish
   - Success summary with next steps and copy-paste commands
   - Timing info (e.g., "done in 0.34s")
   - Verbose mode for debugging (still prompt-first; `--verbose` optional)

7) Tests (where feasible)
   - Unit: artifactId sanitizer, java version parser, template rendering
   - Integration: tempdir scaffold + assertions; Windows CI path optional

---

## Open Questions
- Should `jpm.yaml` own dependency declarations entirely, with Maven/Gradle files derived? (Preferred)
- Do we commit `.jpm` to VCS or treat as generated? (Prototype: commit minimal; revisit later)
- How strict should init be about Java presence? Allow skip with clear guidance vs hard requirement.

---

## Acceptance Criteria (Prototype)
- `jpm init` runs interactively with no required flags and completes on macOS/Linux without errors.
- Produces the exact structure and files described above with a readable `jpm.yaml`.
- Gracefully handles: missing Java, non-empty directory, Windows symlink fallback plan documented.
- User-facing guidance never mentions mvn/gradle; only `jpm` commands are shown.

## Rollout Plan
- Land behind `init` command guarded by "Prototype" banner on start.
- Gather feedback from 3–5 developers; iterate prompts and defaults.
- Add Gradle option stub in UI with "coming soon" note to set expectations.
