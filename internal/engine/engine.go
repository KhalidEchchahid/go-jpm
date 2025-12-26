package engine

import (
	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// BuildResult holds the output of a build operation.
type BuildResult struct {
	ArtifactPath  string   // path to the output JAR
	ClasspathJars []string // dependency JARs for runtime
	Logs          string   // build logs
}

// Engine defines the interface for build engines (native, maven, etc.).
type Engine interface {
	// Name returns the engine identifier.
	Name() string

	// Build compiles and packages the project.
	Build(projectRoot string, manifest *core.Manifest) (*BuildResult, error)

	// ResolveClasspath returns the runtime classpath for the project.
	ResolveClasspath(projectRoot string, manifest *core.Manifest) ([]string, error)
}
