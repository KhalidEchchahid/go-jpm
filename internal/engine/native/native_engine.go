package native

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine"
)

// Engine implements the native JPM build engine.
type Engine struct{}

// New creates a new native engine instance.
func New() *Engine {
	return &Engine{}
}

// Name returns the engine identifier.
func (e *Engine) Name() string {
	return "native"
}

// Build compiles and packages the project using the native toolchain.
func (e *Engine) Build(projectRoot string, manifest *core.Manifest) (*engine.BuildResult, error) {
	jpmDir := filepath.Join(projectRoot, ".jpm")
	outDir := filepath.Join(jpmDir, "out")
	classesDir := filepath.Join(jpmDir, "tmp", "classes")
	srcDir := filepath.Join(projectRoot, "src")

	// Ensure directories exist
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	// Determine artifact name
	artifactID := manifest.Project.ArtifactID
	if artifactID == "" {
		artifactID = "app"
	}
	version := manifest.Project.Version
	if version == "" {
		version = "0.1.0-SNAPSHOT"
	}
	outJar := filepath.Join(outDir, fmt.Sprintf("%s-%s.jar", artifactID, version))

	// Resolve dependencies with transitive closure
	cache := DefaultProjectCache(projectRoot)
	downloader := NewDownloader("", cache)
	resolver := NewResolver(downloader, cache)

	resolved, err := resolver.Resolve(manifest.Dependencies)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Get classpath JARs for compile scope
	classpathJars := ClasspathJars(resolved, "compile")

	// Compile sources
	javaVersion := manifest.Java.Version
	if javaVersion == "" {
		javaVersion = "21"
	}
	compiler := NewCompiler(srcDir, classesDir, javaVersion)
	compiler.SetClasspath(classpathJars)

	compileLog, err := compiler.Compile()
	if err != nil {
		return &engine.BuildResult{Logs: compileLog}, err
	}

	// Package JAR
	packager := NewPackager(classesDir, outJar)
	mainClass := strings.TrimSpace(manifest.App.MainClass)
	if mainClass != "" {
		packager.SetMainClass(mainClass)
	}

	packageLog, err := packager.Package()
	if err != nil {
		return &engine.BuildResult{Logs: compileLog + "\n" + packageLog}, err
	}

	// Save classpath for runtime
	runtimeJars := ClasspathJars(resolved, "compile", "runtime")
	classpathFile := filepath.Join(outDir, "classpath")
	if len(runtimeJars) > 0 {
		_ = os.WriteFile(classpathFile, []byte(strings.Join(runtimeJars, string(os.PathListSeparator))), 0o644)
	} else {
		_ = os.Remove(classpathFile)
	}

	return &engine.BuildResult{
		ArtifactPath:  outJar,
		ClasspathJars: runtimeJars,
		Logs:          compileLog + "\n" + packageLog,
	}, nil
}

// ResolveClasspath returns the runtime classpath for the project.
func (e *Engine) ResolveClasspath(projectRoot string, manifest *core.Manifest) ([]string, error) {
	cache := DefaultProjectCache(projectRoot)
	downloader := NewDownloader("", cache)
	resolver := NewResolver(downloader, cache)

	resolved, err := resolver.Resolve(manifest.Dependencies)
	if err != nil {
		return nil, err
	}

	return ClasspathJars(resolved, "compile", "runtime"), nil
}
