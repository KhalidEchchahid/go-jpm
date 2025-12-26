# Generators (.gitignore & README) Implementation Guide

Date: 2025-11-09T15:52:08.717Z
Owner: Core CLI Team
Status: Implementation Reference

---
## .gitignore Generator

### Goals
- Create or update a project .gitignore with JPM-specific and common Java/IDE entries.
- Idempotent: running multiple times does not duplicate lines.

### Template (ordered)
```
# JPM build artifacts
.jpm/out/
.jpm/logs/
.jpm/cache/

# Maven/Gradle remnants (if any)
target/

# IDE/Editor metadata
.idea/
.classpath
.settings/
.project
*.iml
.vscode/
```

### Algorithm
1. If .gitignore does not exist → write template.
2. If exists → read file, split lines, trim; build set; append any missing template lines in order; preserve user content and spacing; ensure trailing newline.
3. Write back only if changes detected.

### Tests
- First run creates file; second run is no-op (content identical).
- Existing file preserved; missing lines appended once.

---
## README.md Generator

### Goals
- Create a minimal, helpful README for new JPM projects with quickstart and structure.
- Idempotent and user-friendly; prompt if README.md already exists.

### Template (placeholders in {{braces}})
```
# {{project_name}}

A Java project managed by JPM.

## Quickstart
- jpm build
- jpm run
- jpm deps add group:artifact@version

## Structure
- src/ — Your Java sources
- .jpm/ — JPM internals (build, cache, logs, out)
- jpm.yaml — Project manifest

## Running
If not set, JPM will prompt for the main class or detect it.

## Dependencies
Use `jpm deps add org.slf4j:slf4j-api@2.0.16` then `jpm build`.

## Troubleshooting
- Java not found: install JDK 21 (SDKMAN/Brew/winget) or set JAVA_HOME.
- Symlink issues on Windows: JPM falls back to directory creation automatically.
```

### Algorithm
1. Determine placeholders from manifest (name, java.version, app.main_class if present).
2. If README.md exists → prompt overwrite/append/skip; default skip for safety.
3. Write README.md with placeholders substituted; ensure trailing newline.

### Tests
- Creates README.md when absent.
- Prompts and respects choice when present.
- Placeholder substitution verified.
