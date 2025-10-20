# Sprint Manifest – JPM Core Team

**Sprint Window:** 2025-09-15 → 2025-10-02  
**Product Area:** JPM (Java Project Manager) CLI – Go implementation  
**Facilitator:** Engineering Lead

---

## 1. Objective Snapshot

- Deliver a minimal, working JPM CLI capable of listing modules in Maven-based projects.
- Establish the architectural scaffolding for future dependency-management features.

---

## 2. Completed This Sprint

| Area | Deliverable | Evidence | Notes |
|------|-------------|----------|-------|
| Core Domain | `BuildTool` enum with parser and stringifier | `internal/core/buildtool.go` | Validates tool selection and prevents stringly-typed misuse. |
| Interfaces | `ProjectInspector` interface | `internal/core/inspector.go` | Defines contract for inspection features with clear error semantics. |
| Factory Layer | `InspectorFactory` with Maven support | `internal/core/factory.go` | Returns correct inspector instance; fails fast on unsupported tools. |
| Maven Adapter | Regex-based module discovery | `internal/adapters/maven/inspector.go` | Handles comments, whitespace, and missing module blocks; returns ordered module list. |
| CLI Shell | Cobra root command with persistent flags | `cmd/jpm/root.go` | Provides consistent help text and global verbosity/debug toggles. |
| CLI Feature | `module ls` command | `cmd/jpm/module_ls.go` | Accepts optional path, resolves absolute roots, prints indexed module list. |
| Build Output | Single static binary compilation | `go build -o jpm ./cmd/jpm` | Verified binary builds on Linux (Go 1.21). |

---

## 3. Blockers & Challenging Areas

| Item | Impact | Current Mitigation | Follow-up |
|------|--------|--------------------|-----------|
| Robust POM manipulation (writes) | High – needed for `deps add/remove` | Deferred; read-only regex parsing only | Evaluate `encoding/xml` + formatting-preserving layer next sprint. |
| Gradle inspection parity | High – gaps in product promise | Placeholder inspector returns explicit error | Prototype `settings.gradle` parser and/or Gradle CLI integration. |
| Dependency version discovery | Medium – blocks guided dependency add | Not started | Research Maven Central API usage and local caching strategy. |
| Automated testing | Medium – confidence risk | Manual verification only | Add unit + integration tests before expanding feature surface. |
| Error observability | Low | Cobra defaults only | Design structured logging strategy tied to `--debug`. |

---

## 4. In Progress / Deferred

- **Gradle Inspector**: Stub exists but lacks implementation; requires DSL parsing strategy.
- **Project Initializers**: Package scaffolding present (`internal/init`) with no logic.
- **Documentation**: High-level README updated; deeper docs planned separately.

---

## 5. Next Sprint Backlog Candidates

| Priority | Candidate | Rationale | Dependencies |
|----------|-----------|-----------|--------------|
| P0 | `jpm deps ls` (Maven) | Provide immediate value via read-only dependency visibility | Stable POM parsing, CLI command scaffolding. |
| P0 | Gradle module discovery | Close parity gap; avoid disappointing Gradle users | Decide regex vs CLI approach. |
| P1 | Unit tests for core + Maven inspector | Increase confidence and enable regression detection | Choose testing strategy (table tests + golden files). |
| P1 | Telemetry-ready logging abstraction | Ease issue diagnosis in CLI usage | Requires structured logger (zap/log/slog). |
| P2 | Dependency metadata fetcher | Foundation for add/upgrade workflows | Implement Maven Central client + cache. |

---

## 6. Risks & Assumptions

- **Assumption:** Users can supply project root paths; no automatic project discovery yet.
- **Risk:** Regex-based XML parsing may fail on exotic POM structures (profiles, namespaces).
- **Risk:** Lack of Gradle support undermines “unified tool” messaging unless addressed promptly.
- **Risk:** Absence of tests will slow future refactors and integrations.

---

## 7. Action Items

1. Finalize approach for safe POM modifications (DOM vs streaming vs external library).
2. Spike on Gradle settings parsing; compare runtime cost of invoking `gradle projects`.
3. Introduce baseline unit tests covering factory selection and Maven module parsing edge cases.
4. Draft structured logging proposal (verbosity levels + debug dumps).
5. Define acceptance criteria for dependency inspection commands.

---

## 8. Open Questions

- Should JPM maintain formatted diffs when editing build files, or rely on external formatters?
- Do we require offline support? If yes, caching strategy must account for artifact metadata expiration.
- What is the minimum Gradle version we support (Groovy vs Kotlin DSL parity)?

---

## 9. Sprint Health Check

| Dimension | Status | Commentary |
|-----------|--------|------------|
| Scope | 🟢 On Track | MVP goals met; expansion deferred consciously. |
| Quality | 🟡 Needs Attention | No automated tests; error handling minimal but acceptable for prototype. |
| Velocity Predictability | 🟢 Stable | Stories completed as forecast; few surprises. |
| Product Confidence | 🟡 Mixed | Feature works for Maven; story incomplete for Gradle users. |

---

Prepared for the sprint review and planning session.
