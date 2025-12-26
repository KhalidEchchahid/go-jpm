# JPM Lockfile Specification (jpm.lock.yaml)

Date: 2025-11-09
Owner: Core Engine Team
Status: Stable (v1)

---
## Purpose
The lockfile captures an exact, reproducible snapshot of resolved dependencies for a project. It pins artifact versions, checksums, source URLs, and scopes so that builds are deterministic across machines and time, including offline.

---
## Location & Naming
- Path: `<project-root>/jpm.lock.yaml`
- Managed by JPM; human-readable; small diffs; safe to commit to VCS

---
## Versioning
- `lockfile_format`: integer, starting at `1`
- Backward-compat policy: JPM may read older versions; writing upgrades to the latest format

---
## Schema (v1)
```yaml
lockfile_format: 1
created_at: "2025-11-09T15:53:15Z"   # UTC
tool_version: "0.0.354"
resolver: "native"                  # or "maven" for transitional builds
repositories:
  - id: maven-central
    url: https://repo.maven.apache.org/maven2

# Deterministic, GAV-sorted entries
dependencies:
  - gav: "org.slf4j:slf4j-api:2.0.16"
    scope: compile                   # compile|runtime|test|provided
    optional: false
    sha256: "e3b0c44298..."
    size: 30214
    url: "https://repo.maven.apache.org/maven2/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.jar"
    parents:                         # nearest parents that pulled this
      - "com.example:app:1.0.0"

  - gav: "com.google.guava:guava:33.0.0-jre"
    scope: compile
    optional: false
    sha256: "a1b2c3..."
    size: 2873456
    url: "https://repo.maven.apache.org/maven2/com/google/guava/guava/33.0.0-jre/guava-33.0.0-jre.jar"
```

Field notes:
- `parents` is advisory for debugging; does not affect resolution
- `repositories` is informational; fetching may use mirrors configured elsewhere

---
## Determinism Rules
- Entries sorted lexicographically by `gav`
- Timestamps in UTC (Z suffix)
- Stable key order when writing YAML
- Newline-terminated file; no trailing spaces

---
## Generation & Update Rules
- Created/updated on `jpm build` (native engine) or `jpm deps sync`
- Regenerated when manifest dependencies change or when `--update` is passed
- If a dependency entry exists with a checksum, JPM verifies it before use
- Manual edits are ignored at runtime; JPM trusts on-disk values but will fail on checksum mismatch

---
## Integrity & Security
- HTTPS URLs only (HTTP rejected by default)
- Checksums (SHA-256) required for artifacts once fetched; absence triggers computation and write-back
- Mismatch handling: re-fetch once; if still mismatched → fail with remediation note

---
## Offline Behavior
- With `--offline`, JPM uses only cached artifacts referenced by lockfile
- If a locked artifact is missing from cache → fail and list missing GAVs

---
## VCS & Merge Guidance
- Commit `jpm.lock.yaml` to source control
- On merge conflicts: prefer re-run `jpm deps sync` to regenerate deterministically
- Review diffs: expect sorted changes, small, auditable

---
## Example Minimal Lockfile
```yaml
lockfile_format: 1
created_at: "2025-11-09T15:53:15Z"
tool_version: "0.0.354"
resolver: "native"
repositories:
  - id: maven-central
    url: https://repo.maven.apache.org/maven2

dependencies:
  - gav: "com.example:app:1.0.0"
    scope: compile
    optional: false
    sha256: "e3b0c44..."
    size: 0
    url: "https://repo.maven.apache.org/maven2/com/example/app/1.0.0/app-1.0.0.jar"
```

---
## Migration & Compatibility
- Maven-engine projects can coexist; lockfile `resolver: maven` is allowed during transition
- Upgrading JPM rewrites with the latest `tool_version`; format bump increments `lockfile_format`

---
## Validation
- `jpm deps verify` (future): verify all entries present in cache and checksums match
- CI step recommendation: run verify to catch corruption early
