# JPM Quick Reference Card

**Printed**: November 9, 2025  
**Keep by your desk** 📌

---

## 🎯 WHERE TO START

```
1. Read: SPRINT2_START_CHECKLIST.md (10 min)
2. Read: IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md (30 min)
3. Read: CURRENT_STATUS_NOV9.md (20 min)
4. Reference: MASTER_INDEX_NOV9.md (as needed)
```

---

## ❓ QUICK ANSWERS

| Q | A |
|---|---|
| Why pom.xml? | Temporary (Maven bridge). jpm.yaml is source of truth. |
| Completed Sprint 5? | No. Sprint 1 ✅, Sprint 2-6 partial 🟡, Sprint 7-9 planned. |
| Telemetry? | Removed. Zero data collection. |
| .gitignore/README? | Done (Sprint 1). Need to integrate into init (Sprint 2). |
| Native engine? | 60% code written; 40% integration needed. |

---

## 📊 CURRENT STATUS

```
Phase 1: COMPLETE ✅
├─ jpm init, build, run, deps
└─ Maven bridge, jpm.yaml system

Phase 2: PARTIAL 🟡
├─ Sprint 1: Generators ✅ 100%
├─ Sprint 2: Java detection 🟡 70%
├─ Sprint 3: Resolver 🟡 60%
├─ Sprint 4: Fetcher 🟡 40%
├─ Sprint 5: Lockfile ⚠️ 10%
├─ Sprint 6: Compiler 🟡 40%
└─ Sprint 7-9: Planned ⚠️ 0%
```

---

## 🚨 BLOCKERS (FIX THIS WEEK)

1. resolver.DependencyGraph import error → Update imports
2. Java detection not in init.go → Call detector
3. Legacy tests fail → Update fixtures
4. Native resolver not in build → Wire build.go
5. Lockfile not generated → Implement writer

---

## 📅 TIMELINE

```
Nov 9-15:  Fix blockers
Nov 16-22: Sprint 2 (Java + resolver integration)
Dec 1-14:  Sprint 3 (full resolver)
Dec 15-28: Sprint 4-5 (fetcher + lockfile)
Jan 1-15:  Sprint 6 (native compiler default)
Jan 16+:   Sprint 7-9 (polish + release)

Target: v0.2.0 ~end of January 2026
```

---

## 💻 QUICK BUILD

```bash
cd /home/eamonamkassou/Work/go-jpm

# Build
go build ./cmd/jpm

# Test
go test ./...

# Run
./jpm init
./jpm build
./jpm run
```

---

## 📚 BY ROLE

### Project Manager 👤
1. CURRENT_STATUS_NOV9.md
2. IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md
3. PHASE2_BACKLOG_PRIORITIZED.md

### Engineer 👨‍💻
1. SPRINT2_START_CHECKLIST.md § Immediate Actions
2. IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md § Your Sprint
3. NATIVE_ENGINE_ARCHITECTURE.md

### QA 🧪
1. TESTING_STRATEGY.md
2. CURRENT_STATUS_NOV9.md § Known Issues
3. SPRINT2_START_CHECKLIST.md § Fix Tests

### Writer 📖
1. INIT_PROTOTYPE.md
2. NATIVE_ENGINE_ARCHITECTURE.md
3. README update: Add native engine section

---

## 🎯 SPRINT 2 GOALS

- [x] Fix compilation errors
- [x] Integrate Java detection into init
- [x] Wire native resolver into build
- [x] Implement lockfile writer
- [x] Update tests (80%+ passing)
- [x] Update documentation

---

## 💡 KEY FACTS

✅ jpm.yaml is source of truth  
✅ pom.xml will be retired (Spring 6)  
✅ Maven bridge is temporary  
✅ No telemetry (privacy-first)  
✅ Native engine 60% code done  
✅ Phase 1 working perfectly  
✅ Documentation comprehensive  

---

## ⚠️ RISKS

| Risk | Mitigation |
|------|-----------|
| Compilation errors | Fix early; CI check |
| Java detection fails | Test on Windows early |
| Lockfile not deterministic | Sort alphabetic; test 10x |
| Large graphs slow | Profile early; optimize |
| Windows symlink issues | Fallback to copy; test |

---

## 📞 WHAT TO DO IF...

**Build fails?**  
→ Read CURRENT_STATUS_NOV9.md § Compilation Errors

**Tests fail?**  
→ Check SPRINT2_START_CHECKLIST.md § Fix Legacy Tests

**Don't know where to start?**  
→ Read SPRINT2_START_CHECKLIST.md (10 min read)

**Need full context?**  
→ Read MASTER_INDEX_NOV9.md (navigation for 38 files)

**Questions about architecture?**  
→ Read NATIVE_ENGINE_ARCHITECTURE.md

---

## 🏁 SUCCESS METRICS

**After Sprint 2**:
- ✅ Build compiles cleanly
- ✅ 80%+ tests passing
- ✅ Java detection works
- ✅ Native engine option available
- ✅ Lockfile generated

**After Sprint 9**:
- ✅ v0.2.0 ready for release
- ✅ All platforms working
- ✅ Performance targets met

---

## 📍 FILE LOCATIONS

```
Docs:  /home/eamonamkassou/Work/go-jpm/docs/
Code:  /home/eamonamkassou/Work/go-jpm/
Test:  /home/eamonamkassou/Work/go-jpm/project_test/

Key Docs:
├─ SPRINT2_START_CHECKLIST.md ⭐
├─ IMPLEMENTATION_ROADMAP_COMPREHENSIVE.md ⭐
├─ CURRENT_STATUS_NOV9.md ⭐
└─ MASTER_INDEX_NOV9.md ⭐
```

---

**Printed**: 2025-11-09  
**Status**: ✅ READY FOR SPRINT 2  
**Next Review**: Sprint 2 Kickoff (Nov 16)

Keep this card handy! 📌
