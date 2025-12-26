# JPM Phase 2 + Native Engine - Documentation Index

Last Updated: 2025-11-09T16:00:00Z

---
## Quick Navigation

### 🚀 Start Here
1. **DELIVERY_SUMMARY_PHASE2.md** — Overview of all Phase 2 work
2. **PHASE2_BACKLOG_PRIORITIZED.md** — 9 sprints, what to implement next

### 📋 Planning & Strategy
3. **PHASE2_NATIVE_ENGINE_PLAN.md** — Full Phase 2 scope, no telemetry, 9 epics
4. **PHASE2_SPRINT1_COMPLETION.md** — Sprint 1 deliverables (generators) ✅

### 🏗️ Architecture & Design
5. **NATIVE_ENGINE_ARCHITECTURE.md** — System diagram, 7 core components, error taxonomy
6. **RESOLVER_IMPLEMENTATION.md** — BFS traversal, conflict resolution, POM parsing
7. **FETCHER_IMPLEMENTATION.md** — Caching, parallel fetch, retry backoff, offline mode

### 📖 Build Pipeline Guides
8. **COMPILER_IMPLEMENTATION.md** — Javac invocation, source discovery
9. **PACKAGER_IMPLEMENTATION.md** — JAR creation, deterministic manifest
10. **CLASSPATH_IMPLEMENTATION.md** — Scope rules, path assembly

### 🎯 Supporting Specs
11. **GENERATORS_IMPLEMENTATION.md** — .gitignore + README creation
12. **LOCKFILE_SPEC.md** — jpm.lock.yaml v1 format, determinism rules
13. **TESTING_STRATEGY.md** — Unit/integration/E2E, CI matrix

---
## Document Types

### Planning & Backlog (Read First)
- DELIVERY_SUMMARY_PHASE2.md (4 KB)
- PHASE2_NATIVE_ENGINE_PLAN.md (13 KB)
- PHASE2_BACKLOG_PRIORITIZED.md (9.6 KB)
- PHASE2_SPRINT1_COMPLETION.md (4.9 KB)

**Purpose**: Understand scope, prioritization, and current status.

### Architecture & Specification
- NATIVE_ENGINE_ARCHITECTURE.md (19 KB)
- LOCKFILE_SPEC.md (3.7 KB)
- TESTING_STRATEGY.md (3.1 KB)

**Purpose**: High-level design, data formats, validation strategy.

### Implementation Guides (Reference While Coding)
- RESOLVER_IMPLEMENTATION.md (11 KB)
- FETCHER_IMPLEMENTATION.md (13 KB)
- COMPILER_IMPLEMENTATION.md (2.2 KB)
- PACKAGER_IMPLEMENTATION.md (1.6 KB)
- CLASSPATH_IMPLEMENTATION.md (1.4 KB)
- GENERATORS_IMPLEMENTATION.md (2.2 KB)

**Purpose**: Detailed algorithms, data models, error handling.

---
## Implemented & Tested ✅

### Sprint 1 - Generators
- ✅ internal/init/gitignore.go (68 lines)
- ✅ internal/init/readme.go (87 lines)
- ✅ internal/init/gitignore_test.go (5 tests, all passing)

See: PHASE2_SPRINT1_COMPLETION.md

---
## Next Steps by Sprint

### Sprint 2: Install Assist (Next Week)
Read: PHASE2_BACKLOG_PRIORITIZED.md (Sprint 2 section)
Files to create:
- internal/java/detector.go
- internal/java/sdkman.go
- internal/java/homebrew.go

### Sprint 3: Resolver Core
Read: RESOLVER_IMPLEMENTATION.md
Files to create:
- internal/engine/resolver/resolver.go
- internal/engine/resolver/model.go
- internal/engine/resolver/pom_parser.go

### Sprint 4: Fetcher + Cache
Read: FETCHER_IMPLEMENTATION.md
Files to create:
- internal/engine/fetcher/fetcher.go
- internal/engine/fetcher/cache.go

### Sprint 5-9
Follow PHASE2_BACKLOG_PRIORITIZED.md for each sprint.

---
## Key Principles

### Telemetry
✅ REMOVED (per request, 2025-11-09)

### Optimizations
1. **Parallel Fetch**: Worker pool (8 workers by default)
2. **Deterministic Lockfile**: Sorted by GAV, UTC timestamps
3. **Content-Addressed Cache**: SHA-256 keying
4. **Nearest-Wins Resolution**: Maven transitive semantics
5. **Offline-First**: Cache respects --offline flag

### User Experience
- Prompt-first, flags optional
- Graceful degradation (offline mode)
- Clear error messages with remediation
- No tool internals exposed (mvn/gradle hidden)
- HTTPS-only network fetches

---
## Testing Checklist

### Unit Tests (100% Pass)
- ✅ Gitignore generator (create, idempotent, preserve)
- ✅ README generator (create, placeholder substitution)

### Integration Tests (Pending)
- ⏳ Full init workflow (after init.go integration)
- ⏳ Resolver real graph (after Sprint 3)
- ⏳ Fetcher with Maven Central (after Sprint 4)

### CI/CD (Pending)
- ⏳ GitHub Actions matrix (Ubuntu, macOS, Windows)
- ⏳ Java 17, 21, 23 versions

---
## File Manifest

Total Phase 2 Deliverables:
- **13 Documentation Files** (~100 KB)
- **3 Code Files** (294 lines)
- **5 Unit Tests** (all passing)

All files in `/home/eamonamkassou/Work/go-jpm/docs/` and `/internal/init/`.

---
## How to Use This Index

**I'm starting a new sprint:**
1. Find your sprint number in "Next Steps by Sprint"
2. Read the corresponding implementation guide
3. Check PHASE2_BACKLOG_PRIORITIZED.md for task list
4. Follow the guide's implementation checklist

**I need to understand resolver:**
1. Read RESOLVER_IMPLEMENTATION.md
2. Study the algorithm section
3. Review the example flow
4. Check the data model diagrams

**I want to see what's done:**
1. Read DELIVERY_SUMMARY_PHASE2.md
2. Check PHASE2_SPRINT1_COMPLETION.md
3. Review test results in this index

---
## Quick Commands

```bash
# Run Sprint 1 tests
cd /home/eamonamkassou/Work/go-jpm
go test ./internal/init/... -v

# View all docs
ls -lh docs/PHASE2_*.md docs/NATIVE_ENGINE_*.md docs/*_IMPLEMENTATION.md

# Read a specific guide
cat docs/RESOLVER_IMPLEMENTATION.md | less
```

---
## Questions & Answers

**Q: Is telemetry included?**
A: No. Removed 2025-11-09 per request.

**Q: Can I parallelize sprints?**
A: Yes. Sprint 2 (Install) and Sprint 3 (Resolver) can happen simultaneously.

**Q: What's the MVP timeline?**
A: ~9 weeks (Sprints 1-9). Sprint 1 complete. Sprints 2-9 estimated 8 weeks.

**Q: Is the native engine stable?**
A: In design phase. Keep engine=maven default until stable. Feature-flag native.

**Q: Can I start coding?**
A: Yes! Sprint 2 is ready. See PHASE2_BACKLOG_PRIORITIZED.md for tasks.

---
## Version History

| Date | Change |
|------|--------|
| 2025-11-09 | ✅ Sprint 1 complete, documentation delivered |
| 2025-11-09 | ✅ Telemetry removed |
| 2025-11-09 | ✅ INDEX_PHASE2.md created |

---
## Contact & Questions

For clarifications on any document, refer to:
- Implementation Guides for "how"
- PHASE2_BACKLOG_PRIORITIZED.md for "what's next"
- DELIVERY_SUMMARY_PHASE2.md for "what's done"

