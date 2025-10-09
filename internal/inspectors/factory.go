package inspectors

import (
	"fmt"

	"github.com/KhalidEchchahid/go-jpm/internal/adapters/gradle"
	"github.com/KhalidEchchahid/go-jpm/internal/adapters/maven"
	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// Factory creates project inspectors for different build tools. It acts as the
// seam between the CLI and adapter implementations, keeping construction logic
// in one place.
type Factory struct{}

// NewFactory instantiates a fresh Factory instance.
func NewFactory() *Factory {
	return &Factory{}
}

// ForTool returns a ProjectInspector implementation for the provided build tool
// so callers can interrogate projects without caring about the underlying
// build-system specifics.
func (f *Factory) ForTool(tool core.BuildTool) (core.ProjectInspector, error) {
	switch tool {
	case core.Maven:
		return maven.NewMavenProjectInspector(), nil
	case core.Gradle:
		return gradle.NewGradleProjectInspector(), nil
	default:
		return nil, fmt.Errorf("unsupported build tool: %s", tool.String())
	}
}
