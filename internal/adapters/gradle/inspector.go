// Package gradle contains the (currently stubbed) ProjectInspector
// implementation for Gradle builds. The adapter will be expanded in a future
// sprint once Gradle parsing is prioritized.
package gradle

import (
	"fmt"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// GradleProjectInspector implements ProjectInspector for Gradle projects.
type GradleProjectInspector struct{}

// NewGradleProjectInspector creates a new Gradle project inspector.
func NewGradleProjectInspector() *GradleProjectInspector {
	return &GradleProjectInspector{}
}

// ListModules returns a list of module names from the Gradle project. Gradle
// support is not yet implemented, so the method currently returns an error.
func (i *GradleProjectInspector) ListModules(projectRoot string) ([]string, error) {
	return nil, fmt.Errorf("gradle support not yet implemented")
}

// ListDependencies returns declared dependencies from Gradle build files. Like
// ListModules, this is a placeholder until the Gradle adapter is implemented.
func (i *GradleProjectInspector) ListDependencies(projectRoot string) ([]core.Dependency, error) {
	return nil, fmt.Errorf("gradle support not yet implemented")
}

// DependencyTree provides a hierarchical view of Gradle dependencies. The
// Gradle adapter is not yet implemented, so this currently returns an error to
// keep CLI feedback consistent across commands.
func (i *GradleProjectInspector) DependencyTree(projectRoot string) (*core.DependencyTree, error) {
	return nil, fmt.Errorf("gradle support not yet implemented")
}
