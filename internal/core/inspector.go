package core

// ProjectInspector defines the interface for inspecting project structures.
// Adapters implement this contract to supply build-tool-specific knowledge
// while keeping the CLI oblivious to Maven vs Gradle nuances.
type ProjectInspector interface {
	// ListModules returns a list of module names in the project.
	ListModules(projectRoot string) ([]string, error)

	// ListDependencies returns the declared dependencies in the project
	// definition. Read operations rely on this to render "jpm deps show".
	ListDependencies(projectRoot string) ([]Dependency, error)
}
