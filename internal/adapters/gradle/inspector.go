package gradle

import "fmt"

// GradleProjectInspector implements ProjectInspector for Gradle projects
type GradleProjectInspector struct{}

// NewGradleProjectInspector creates a new Gradle project inspector
func NewGradleProjectInspector() *GradleProjectInspector {
	return &GradleProjectInspector{}
}

// ListModules returns a list of module names from the Gradle project
func (i *GradleProjectInspector) ListModules(projectRoot string) ([]string, error) {
	return nil, fmt.Errorf("gradle support not yet implemented")
}