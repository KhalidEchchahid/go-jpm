package native

import (
	"fmt"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// ResolvedDependency holds a resolved artifact with its transitive closure.
type ResolvedDependency struct {
	Artifact   Artifact
	Scope      string
	Optional   bool
	Exclusions map[string]bool // "groupId:artifactId" -> excluded
	LocalPath  string          // path to JAR in cache
}

// Resolver performs transitive dependency resolution.
type Resolver struct {
	downloader *Downloader
	cache      *Cache
	resolved   map[string]*ResolvedDependency // coordinate -> resolved
	visited    map[string]bool                // track cycles
}

// NewResolver creates a resolver with the given downloader and cache.
func NewResolver(downloader *Downloader, cache *Cache) *Resolver {
	return &Resolver{
		downloader: downloader,
		cache:      cache,
		resolved:   make(map[string]*ResolvedDependency),
		visited:    make(map[string]bool),
	}
}

// Resolve takes direct dependencies from the manifest and returns the full
// transitive closure as a flat list of resolved dependencies.
func (r *Resolver) Resolve(direct []core.Dependency) ([]*ResolvedDependency, error) {
	for _, dep := range direct {
		artifact := Artifact{
			GroupID:    strings.TrimSpace(dep.GroupID),
			ArtifactID: strings.TrimSpace(dep.ArtifactID),
			Version:    strings.TrimSpace(dep.Version),
			Classifier: strings.TrimSpace(dep.Classifier),
			Packaging:  strings.TrimSpace(dep.Type),
		}
		if artifact.Packaging == "" {
			artifact.Packaging = "jar"
		}

		scope := strings.TrimSpace(dep.Scope)
		if scope == "" {
			scope = "compile"
		}

		exclusions := make(map[string]bool)

		if err := r.resolveTransitive(artifact, scope, dep.Optional, exclusions, 0); err != nil {
			return nil, err
		}
	}

	// Collect results
	result := make([]*ResolvedDependency, 0, len(r.resolved))
	for _, rd := range r.resolved {
		result = append(result, rd)
	}
	return result, nil
}

// resolveTransitive recursively resolves a single artifact and its dependencies.
func (r *Resolver) resolveTransitive(artifact Artifact, scope string, optional bool, parentExclusions map[string]bool, depth int) error {
	// Prevent infinite loops
	if depth > 50 {
		return fmt.Errorf("dependency resolution depth exceeded for %s", artifact.Coordinate())
	}

	coord := artifact.Coordinate()

	// Check if excluded by parent
	excludeKey := artifact.GroupID + ":" + artifact.ArtifactID
	if parentExclusions[excludeKey] {
		return nil
	}

	// Skip if already resolved (first wins - nearest definition)
	if _, exists := r.resolved[coord]; exists {
		return nil
	}

	// Cycle detection
	if r.visited[coord] {
		return nil
	}
	r.visited[coord] = true
	defer func() { delete(r.visited, coord) }()

	// Skip non-jar packaging for transitive resolution (e.g., pom, war)
	if artifact.Packaging != "" && artifact.Packaging != "jar" {
		return nil
	}

	// Download artifact JAR
	localPath, err := r.downloader.EnsureArtifact(artifact)
	if err != nil {
		// Some artifacts may not have a JAR (e.g., pom-only), continue gracefully
		localPath = ""
	}

	// Record this dependency
	r.resolved[coord] = &ResolvedDependency{
		Artifact:   artifact,
		Scope:      scope,
		Optional:   optional,
		Exclusions: parentExclusions,
		LocalPath:  localPath,
	}

	// Download and parse POM for transitive dependencies
	pomPath, err := r.downloader.EnsurePOM(artifact)
	if err != nil {
		// POM not found is not fatal; just no transitives
		return nil
	}

	pom, err := ParsePOM(pomPath)
	if err != nil {
		// Parse error is not fatal; skip transitives
		return nil
	}

	// Resolve transitive dependencies
	for _, dep := range pom.ResolveDependencies() {
		// Skip optional dependencies
		if dep.IsOptional() {
			continue
		}

		// Skip test/provided scope in transitives
		depScope := strings.TrimSpace(dep.Scope)
		if depScope == "test" || depScope == "provided" || depScope == "system" {
			continue
		}

		// Build exclusion set for this dependency
		childExclusions := make(map[string]bool)
		for k, v := range parentExclusions {
			childExclusions[k] = v
		}
		for _, excl := range dep.Exclusions {
			key := excl.GroupID + ":" + excl.ArtifactID
			childExclusions[key] = true
		}

		childArtifact := Artifact{
			GroupID:    dep.GroupID,
			ArtifactID: dep.ArtifactID,
			Version:    dep.Version,
			Classifier: dep.Classifier,
			Packaging:  dep.Type,
		}
		if childArtifact.Packaging == "" {
			childArtifact.Packaging = "jar"
		}

		// Skip if version is still unresolved
		if childArtifact.Version == "" || strings.HasPrefix(childArtifact.Version, "${") {
			continue
		}

		if err := r.resolveTransitive(childArtifact, depScope, false, childExclusions, depth+1); err != nil {
			// Log but don't fail on transitive resolution errors
			continue
		}
	}

	return nil
}

// ClasspathJars returns a list of JAR paths for compilation/runtime.
// Scopes can filter which dependencies to include.
func ClasspathJars(deps []*ResolvedDependency, includeScopes ...string) []string {
	scopeSet := make(map[string]bool)
	for _, s := range includeScopes {
		scopeSet[s] = true
	}
	if len(scopeSet) == 0 {
		scopeSet["compile"] = true
		scopeSet["runtime"] = true
	}

	var jars []string
	seen := make(map[string]bool)
	for _, dep := range deps {
		if !scopeSet[dep.Scope] {
			continue
		}
		if dep.LocalPath == "" {
			continue
		}
		if seen[dep.LocalPath] {
			continue
		}
		seen[dep.LocalPath] = true
		jars = append(jars, dep.LocalPath)
	}
	return jars
}
