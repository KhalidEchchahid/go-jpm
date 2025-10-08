package core

// ProjectInspector defines the interface for inspecting project structures
type ProjectInspector interface {
	// ListModules returns a list of module names in the project
	ListModules(projectRoot string) ([]string, error)
}