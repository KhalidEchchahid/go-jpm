# Sprint 2 Start Checklist & Quick Answers

**Date**: 2025-11-09  
**Target Start**: Nov 16, 2025  
**Owner**: Core CLI Team  

---

## QUICK ANSWERS TO USER QUESTIONS

### Q1: "Why are we still using pom.xml? Aren't we using jpm.yaml?"

**Short Answer**: YES, we use jpm.yaml. pom.xml is **temporary and derived** from jpm.yaml.

**Long Answer**:
- jpm.yaml is the **source of truth** (user edits this)
- pom.xml is **generated** (derived for Maven bridge)
- Maven bridge is **transitional** (Sprint 6 replaces with native compiler)
- After Sprint 6: pom.xml gone, only jpm.yaml + native build

**Timeline**:
- NOW: pom.xml generated from jpm.yaml (Maven fallback)
- Sprint 6: Native compiler ready; retire Maven
- After Sprint 6: Only jpm.yaml; no pom.xml

---

### Q2: "We've already completed sprint 5 no?"

**Short Answer**: NO. Current status:
- ✅ Sprint 1: COMPLETE (generators)
- 🟡 Sprint 2: 70% (Java detection done, not integrated)
- 🟡 Sprint 3: 60% (resolver exists, not integrated)
- 🟡 Sprint 4: 40% (fetcher skeleton)
- ⚠️ Sprint 5: 10% (lockfile spec only)
- ⚠️ Sprints 6-9: Planned but not started

**Why Confusion**?
- Code for Sprints 2-6 was written but not **integrated**
- Tests mostly failing due to legacy references
- Need Sprint 2 completion to wire everything

---

### Q3: "Are we removing telemetry?"

**Short Answer**: YES, already removed. 

**Status**:
- ✅ No telemetry code in codebase
- ✅ No analytics libraries
- ✅ No data collection planned
- ✅ Phase 2 plan explicitly excludes it

---

### Q4: "What about .gitignore and README generator?"

**Short Answer**: DONE (Sprint 1). Files exist and tested.

**Details**:
- ✅ `internal/init/gitignore.go`: Idempotent .gitignore generation
- ✅ `internal/init/readme.go`: Template-based README generation
- ✅ 5 unit tests (all passing)
- ⚠️ NOT integrated into `jpm init` flow yet (Sprint 2 task)

---

### Q5: "What's the optimal strategy for fetching dependencies?"

**Short Answer**: Parallelized, cached, offline-capable.

**Details**: See IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md § "NATIVE ENGINE: DEPENDENCY FETCHING STRATEGY"

**Quick Summary**:
1. Parse manifest dependencies
2. BFS graph expansion (fetch POMs in parallel)
3. Resolve version conflicts (nearest-wins)
4. Generate deterministic lockfile
5. Parallel artifact fetch (worker pool)
6. Cache with SHA256 keys
7. Offline: skip network; error if cache miss

**Performance Target**:
- Cold resolve: <5s for 50-dep tree
- Warm resolve: <1s (cache hits)
- Offline: instant (cache only)

---

## IMMEDIATE ACTION ITEMS (Do This Week)

### 1. Fix Build Compilation Errors [Priority: CRITICAL]

**Issue**: 
```
internal/engine/build/builder.go:53:18: undefined: resolver.DependencyGraph
```

**Fix**:
```bash
# Check what exists in resolver
grep -n "type.*Graph" internal/engine/resolver/*.go

# Update builder.go to import correctly
# File: internal/engine/build/builder.go, Line 53
# Change: resolver.DependencyGraph
# To: using the correct exported type from model.go
```

**Owner**: Sprint 2 lead  
**Time**: 30 minutes  
**Acceptance**: `go build ./cmd/jpm` succeeds

---

### 2. Integrate Java Detection into Init [Priority: HIGH]

**Current State**:
- ✅ `internal/java/detector.go` written and tested
- ✅ `internal/java/sdkman.go` ready
- ✅ `internal/java/homebrew.go` ready
- ❌ NOT called from `cmd/jpm/init.go`

**Required Changes**:
```go
// File: cmd/jpm/init.go
// Add after prompting for template:

java, err := java.DetectJava()
if err != nil {
    // Offer install
    java.OfferInstall(manifest.Java.Version)
}
```

**Owner**: Sprint 2 lead  
**Time**: 1-2 hours  
**Acceptance**: `jpm init` detects Java and offers install if missing

---

### 3. Fix Legacy Test Failures [Priority: HIGH]

**Issue**: Tests reference old `java-legacy/` paths

**Fix Options**:
- A) Update tests to use new project structure
- B) Skip legacy tests; add new integration tests

**Recommendation**: Option A (better coverage)

**Owner**: QA / Test lead  
**Time**: 2-3 hours  
**Acceptance**: `go test ./cmd/jpm` passes (or skip legacy explicitly)

---

### 4. Wire Native Resolver into Build [Priority: HIGH]

**Current State**:
- ✅ Resolver exists in `internal/engine/resolver/`
- ❌ Not called from `cmd/jpm/build.go`

**Required Changes**:
```go
// File: cmd/jpm/build.go
// After loading manifest:

if manifest.Engine == "native" {
    // Use native resolver
    graph, err := resolver.Resolve(manifest)
    // Use graph for classpath
} else {
    // Use Maven bridge (current)
}
```

**Owner**: Sprint 2 lead  
**Time**: 1-2 hours  
**Acceptance**: `jpm build` can use native engine option

---

### 5. Implement Lockfile Writer [Priority: MEDIUM]

**Issue**: Resolution succeeds but lockfile not generated

**File to Create**: `internal/lockfile/writer.go`

**Template**:
```go
package lockfile

import (
    "github.com/KhalidEchchahid/go-jpm/internal/engine/resolver"
    "gopkg.in/yaml.v3"
)

func WriteLockfile(resolved []resolver.Dependency, path string) error {
    // Sort by GAV
    // Add SHA256 checksums
    // Write jpm.lock.yaml
}
```

**Owner**: Sprint 5 lead (move to Sprint 2 if urgent)  
**Time**: 1-2 hours  
**Acceptance**: `jpm build` generates jpm.lock.yaml

---

## SPRINT 2 EXECUTION PLAN

### Week 1: Setup & Integration (Nov 16-22)

**Monday-Tuesday**:
- [ ] Review IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md as team
- [ ] Fix compilation errors
- [ ] Run `go build ./cmd/jpm` successfully
- [ ] Update tests

**Wednesday-Thursday**:
- [ ] Integrate Java detection into init flow
- [ ] Test Java detection + install prompts
- [ ] Update jpm.yaml schema to include Java selection

**Friday**:
- [ ] Wire native resolver into build
- [ ] Test native resolution with sample project
- [ ] Prep Sprint 2 summary

### Week 2: Polish & Completion (Nov 23-29)

**Monday-Tuesday**:
- [ ] Complete lockfile writer
- [ ] Test full resolution → lockfile pipeline
- [ ] Update logging

**Wednesday-Thursday**:
- [ ] Add offline flag support
- [ ] Test offline mode with primed cache
- [ ] Update error messages

**Friday**:
- [ ] Sprint 2 acceptance tests
- [ ] Documentation update
- [ ] Prepare Sprint 3 kickoff

---

## DELIVERABLES CHECKLIST (Sprint 2)

### Code
- [ ] `internal/java/detector.go` integrated into init
- [ ] `internal/java/sdkman.go` working (tested on Linux/macOS)
- [ ] `internal/java/homebrew.go` working (tested on macOS)
- [ ] `cmd/jpm/build.go` updated to support engine selection
- [ ] `internal/lockfile/writer.go` implemented
- [ ] All compilation errors fixed
- [ ] 80%+ tests passing

### Documentation
- [ ] SPRINT2_COMPLETION.md created
- [ ] README updated with native engine info
- [ ] Changelog updated
- [ ] Next sprint plan (Sprint 3) documented

### Testing
- [ ] Unit tests: Java detection (15+ tests)
- [ ] Integration tests: init → Java assist → build (5+ tests)
- [ ] E2E test: Full workflow on sample project
- [ ] Offline mode test (4+ tests)
- [ ] Windows: symlink fallback tested

### Acceptance Criteria
- [x] `jpm init` detects Java and offers install
- [x] Java version stored in jpm.yaml
- [x] `jpm build` can use native engine
- [x] Lockfile generated on successful build
- [x] All 80%+ tests passing
- [x] Zero compilation errors
- [x] Documentation complete

---

## SUCCESS METRICS

### After Sprint 2
- ✅ Build command: Both engine options available (maven | native)
- ✅ Init command: Auto-installs JDK if missing
- ✅ Manifest: Full jpm.yaml → native build pipeline working
- ✅ Tests: 80%+ passing; legacy tests migrated or deprecated
- ✅ Documentation: Updated with Sprint 2 changes

### Performance Targets
- JDK detection: <1s
- Project initialization: <3s
- Full build: <10s cold, <2s warm

---

## RESOURCES & REFERENCES

### Documentation
1. **IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md** (This session)
   - Complete 9-sprint plan with task-level detail
   - Dependency fetching strategy
   - Risk mitigation

2. **CURRENT_STATUS_NOV9.md** (This session)
   - Current implementation status
   - What's done vs. what's needed
   - Known issues & blockers

3. **PHASE2_NATIVE_ENGINE_PLAN.md** (Existing)
   - High-level Phase 2 scope
   - Epic definitions

4. **PHASE2_BACKLOG_PRIORITIZED.md** (Existing)
   - Detailed task list for Sprints 2-9

### Code References
- Java detection: `internal/java/*.go`
- Resolver: `internal/engine/resolver/*.go`
- Fetcher: `internal/engine/fetcher/*.go`
- Compiler: `internal/engine/compiler/*.go`
- Init flow: `cmd/jpm/init.go`
- Build flow: `cmd/jpm/build.go`

### Testing
- Sample project: `project_test/` (jpm.yaml + Main.java)
- Run tests: `go test ./...`
- Build binary: `go build ./cmd/jpm`

---

## RISKS & MITIGATIONS

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Java detection fails on new platform | Blocks init | Test on Windows CI early; add platform detection tests |
| Resolver performance on large graphs | Performance issue | Benchmark on 100-node graph; optimize BFS |
| Lockfile determinism not achieved | Builds not reproducible | Strict alphabetic sort; deterministic timestamps; test 10x |
| Offline mode complexity | Feature creep | Keep --offline simple; cache-only logic |
| Test migration effort | Sprint delay | Start early; prioritize critical tests |

---

## DECISION LOG (Sprint 2 Topics)

### Decision: Support Both Engines in Build
**Rationale**: Maven bridge as fallback while native stabilizes  
**Implementation**: `if manifest.Engine == "native"` in build.go  
**Revisit**: Sprint 6 (retire Maven when native mature)

### Decision: Java Detection Async (Not Blocking)
**Rationale**: User can skip JDK install; init continues  
**Implementation**: Prompt → offer install → skip option  
**Revisit**: Never (user choice respected)

### Decision: Lockfile Regeneration Policy
**Rationale**: Only regenerate if manifest changed or --update flag  
**Implementation**: Compare manifest hash vs lockfile  
**Revisit**: Never (clear and simple)

---

## SIGN-OFF CHECKLIST

**Before Starting Sprint 2**:
- [ ] Team reviewed IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md
- [ ] Team reviewed CURRENT_STATUS_NOV9.md
- [ ] Roles assigned (lead, QA, docs)
- [ ] Blockers understood
- [ ] Resources allocated
- [ ] Sprint goal agreed: "Native resolver integrated + Java assist working"

**End of Sprint 2**:
- [ ] All checklist items completed
- [ ] 80%+ tests passing
- [ ] Documentation updated
- [ ] Sprint 2 summary created
- [ ] Sprint 3 readiness confirmed

---

## CONTACT & ESCALATION

**Sprint 2 Lead**: [Assigned]  
**QA Lead**: [Assigned]  
**Blocker Escalation**: DM lead if:
- Compilation errors not resolved by Wed
- Platform-specific issues arise
- Performance targets missed

---

**Document Version**: 1.0  
**Created**: 2025-11-09T17:02:51Z  
**Status**: 🟢 Ready for Sprint 2 Kickoff  
**Next Review**: Sprint 2 Standup (Nov 16, 2025)
