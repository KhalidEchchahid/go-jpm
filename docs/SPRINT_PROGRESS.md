# Sprint Progress – JPM Init & Engine Prototype

Timestamp: 2025-10-30T09:25:07.954Z
Status: In progress (Phase 0–1)
Owner: Core CLI/Engine

---

## Executive Summary
We established jpm.yaml as the single source of truth and delivered an end-to-end prototype for deps → build → run using a hidden engine, with Maven-backed build as a fallback and a native path planned. The UX is prompt-first and Maven/Gradle are never surfaced to the user. We validated the flow with smoke tests; remaining work focuses on native builds, run UX, Windows symlink handling, and better error/logging.

---

## What’s Implemented
- Manifest & Schema
  - YAML manifest loader/saver (internal/core/manifest_yaml.go)
  - Manifest includes: name, build_tool, engine, java.version, project (group/artifact/version), app.main_class, dependencies[]
- Engine API Scaffold
  - internal/core/engine.go: Engine, Context, BuildPlan/Result, RunSpec, TestSpec, DepsOp/Result, Manifest types
- CLI Commands (manifest-first)
  - deps ls: reads jpm.yaml and lists dependencies (fallback to inspector if missing manifest)
  - deps add: updates jpm.yaml; interactive prompts for group/artifact/version; coordinate arg supported (group:artifact@version)
  - build: renders .jpm/maven/pom.xml from manifest; ensures .jpm/maven/src/main/java → ./src symlink; builds via hidden mvn if present; else produces placeholder jar in .jpm/out; logs to .jpm/logs
  - run: ensures jar (build if missing); prompts for and persists app.main_class; launches `java -cp <jar> <main>`
- Init UX
  - Next steps now reference `jpm build` and `jpm run`; generated manifest includes engine: maven and app.main_class: Main
- Docs
  - docs/INIT_PROTOTYPE.md updated: prompt-first UX, JPM Engine section, dependency schema v0, ASCII pipeline, Phase 0–1 plan and acceptance criteria

Paths touched:
- cmd/jpm/{deps_add.go,deps_ls.go,build.go,run.go,init.go}
- internal/core/{manifest_yaml.go,engine.go}
- docs/INIT_PROTOTYPE.md

---

## Verification / Demos
- Smoke test (local temp project `_jpm_proto_test`):
  - deps ls → “(none)”
  - deps add com.google.guava:guava@33.0.0-jre → added to manifest
  - deps ls → shows `com.google.guava:guava:33.0.0-jre`
  - build → creates `.jpm/out/app-0.1.0-SNAPSHOT.jar` (placeholder if mvn absent; real jar when mvn available)
  - run → prompts for main class if not set; uses manifest `app.main_class` afterwards

---

## Blockers / Risks
- Native engine (javac/jar + dependency resolver) not implemented; currently relies on mvn when present or creates placeholder jar
- Main-class auto-detection not implemented; prompt only
- Windows symlink limitations; need junction fallback and reliable path resolution
- Error translation/logging: need consistent mapping of engine errors to user-friendly messages and log storage in .jpm/logs
- Render-on-build and caching strategy: incremental updates, engine pinning/lock file not yet implemented
- Testing coverage is low; need unit/integration tests for manifest, build, and run flows

Mitigations
- Short-term: keep mvn-backed build as hidden engine (pinned/validated) while native engine is built
- Add junction fallback and capability detection on Windows
- Implement log files by default and expose `--verbose` for engine details

---

## Next Steps (Phase 1 completion → Phase 2 start)
1) Native Build MVP
   - Resolve classpath from manifest deps (no transitive resolution initially)
   - Compile with javac; package jar with `Main-Class` when app.main_class set
   - Emit logs to .jpm/logs and artifacts to .jpm/out
2) Run UX polish
   - Auto-detect main class from compiled classes when possible; prompt fallback
   - Pass-thru program args: `jpm run -- arg1 arg2`
3) Windows compatibility
   - Symlink → junction fallback; tests on Windows CI if available
4) Error & Logging
   - Normalize error messages; `--verbose` reveals engine/stdout; write logs consistently
5) Manifest ops
   - Validate dependency entries (schema v0); enforce scopes {compile,test} with defaults
6) Tests
   - Unit: manifest parser, pom renderer, path helpers
   - Integration: build/run roundtrip on sample project with hello world

---

## Decisions
- UX: JPM-only commands; do not surface mvn/gradle; `.jpm` stays hidden for users
- Source of truth: jpm.yaml owns dependencies; tool files are generated on build
- Engine: start with hidden mvn as a bridge; target native engine ASAP to remove mvn dependency

---

## Open Questions
- Should we pin/ship a mvn distribution inside `.jpm/tools` for reproducibility, or require system mvn if present?
- Do we commit any `.jpm` files or keep all generated? (leaning: generated only; commit manifest)
- Online version resolution for deps add (Maven Central API) in prototype vs next sprint?

---

## Timeline & Owners (proposed)
- Native build MVP (javac/jar), logging, Windows fallback: 1 sprint
- Run UX (auto-detect main, args pass-thru) + tests: 0.5 sprint
- Owners: CLI (commands/prompt), Engine (build/run), Windows (FS/junction), QA (tests)

---

## Appendix: Commands
- Build: `jpm build`
- Run: `jpm run`
- Deps: `jpm deps ls`, `jpm deps add group:artifact@version`
- Init: `jpm init` (interactive)
