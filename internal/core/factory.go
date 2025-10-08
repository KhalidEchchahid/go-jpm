package core

import (
	"fmt"
	"github.com/hicham-amazigh/jpm/internal/adapters/maven"
)

// InspectorFactory creates inspectors for different build tools
type InspectorFactory struct{}

// NewInspectorFactory creates a new inspector factory
func NewInspectorFactory() *InspectorFactory {
	return &InspectorFactory{}
}

// ForTool returns an inspector for the given build tool
func (f *InspectorFactory) ForTool(tool BuildTool) (ProjectInspector, error) {
	switch tool {
	case Maven:
		return maven.NewMavenProjectInspector(), nil
	case Gradle:
		return nil, fmt.Errorf("gradle support not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported build tool: %s", tool.String())
	}
}