# JPM – Java Project Manager

**Unified Dependency & Project Management CLI for Maven & Gradle**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Overview

**JPM (Java Project Manager)** is an **open-source CLI tool** that simplifies managing Java projects.
It unifies workflows across build systems (**Maven** and **Gradle**) and provides commands to:

* Manage dependencies (**add, remove, upgrade, downgrade, show**)
* Inspect projects (**list modules, display dependency tree, detect unused deps**)
* Initialize new projects with common frameworks (**Spring Boot, Micronaut, Dropwizard, Plain Java**)
* Package as a **single binary** for easy distribution

---

## Features

* **Dependency Management**

    * `jpm deps show` → Display declared dependencies *(Maven support today)*
    * *(planned)* `jpm deps add` → Add dependencies safely (with version selection & backups)
    * *(planned)* `jpm deps remove` → Remove dependencies (with usage checks)
    * *(planned)* `jpm deps upgrade` / `downgrade` → Bump dependency versions
    * *(planned)* `jpm deps tree` → View full dependency tree

* **Module Management**

    * `jpm module find` → List modules in multi-module projects *(Maven only)*

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
jpm module find --build-tool maven .
```

Show declared dependencies:

```bash
jpm deps show --build-tool maven .
```

---

## 🛠 Tech Stack

* **Go 1.21+**
* **Cobra** for CLI framework
* **Standard library** for XML/JSON parsing

---

## License

This project is released under the [MIT License](LICENSE).
