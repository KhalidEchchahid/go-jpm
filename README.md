# JPM – Java Project Manager

**Unified Dependency & Project Management CLI for Maven & Gradle**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Overview

**JPM (Java Project Manager)** is an **open-source CLI tool** that simplifies managing Java projects.
It unifies workflows across build systems (**Maven** and **Gradle**) and provides commands to:

* Manage dependencies (**add, remove, upgrade, downgrade, ls**)
* Inspect projects (**list modules, display dependency tree, detect unused deps**)
* Initialize new projects with common frameworks (**Spring Boot, Micronaut, Dropwizard, Plain Java**)
* Package as a **single binary** for easy distribution

---

## Features

* **Dependency Management**

    * `jpm deps ls` → Display declared dependencies *(Maven support today)*
    * `jpm deps tree` → View full dependency tree *(Maven support today)*
    * `jpm deps add` → Add dependencies safely (Maven-only preview with dry-run support)
    * *(planned)* `jpm deps remove` → Remove dependencies (with usage checks)
    * *(planned)* `jpm deps upgrade` / `downgrade` → Bump dependency versions

* **Module Management**

    * `jpm module ls` → List modules in multi-module projects *(Maven only)*

* **Project Initialization** *(planned)*

    * `jpm init java` → Bootstrap plain Java project
    * `jpm init spring` → Generate Spring Boot project
    * `jpm init micronaut` → Generate Micronaut project
    * `jpm init dropwizard` → Generate Dropwizard project

* **Cross-Build Support**

    * Maven and Gradle adapters via pluggable architecture
    * Designed for future expansion (SBT, Bazel…)

* **Distribution**

    * Single binary executable (`./jpm`)

> 🔎 Command naming follows the [CLI Command Conventions](docs/CLI_COMMAND_CONVENTIONS.md) guide so that read-only actions consistently use `ls`.

---

## Project Structure

```
jpm/
├─ cmd/jpm/               # CLI entry point
├─ internal/
│   ├─ core/              # Core business logic & APIs
│   ├─ adapters/
│   │   ├─ maven/         # Maven adapter (POM parsing & editing)
│   │   └─ gradle/        # Gradle adapter (build.gradle support)
│   ├─ inspectors/        # Factory for resolving build tool inspectors
│   └─ init/              # Project initializers (Spring, Micronaut…)
├─ legacy-java/           # Original Java implementation
├─ go.mod                 # Go module definition
├─ go.sum                 # Go dependencies
└─ README.md
```

---

## Installation

### Option 1: Install from Source

```bash
git clone https://github.com/KhalidEchchahid/go-jpm.git
cd jpm
go build -o jpm ./cmd/jpm
./jpm --help
```

### Option 2: Install Pre-built Binary

Coming soon

---

## Usage Examples

List project modules:

```bash
jpm module ls --build-tool maven .
```

List declared dependencies:

```bash
jpm deps ls --build-tool maven .
```

Show the dependency tree (requires Maven on PATH):

```bash
jpm deps tree --build-tool maven .
```

Add a dependency (auto-resolves the latest version when omitted):

```bash
jpm deps add com.squareup.okhttp3:okhttp --build-tool maven --scope test
```

Preview the change without touching pom.xml:

```bash
jpm deps add org.assertj:assertj-core --dry-run
```

Discover available versions before choosing one:

```bash
jpm deps add org.junit.jupiter:junit-jupiter --list-versions
```

---

## 🛠 Tech Stack

* **Go 1.21+**
* **Cobra** for CLI framework
* **Standard library** for XML/JSON parsing

---

## License

This project is released under the [MIT License](LICENSE).
