// Package maven contains the ProjectInspector implementation for Apache Maven
// projects. It parses pom.xml files to surface module lists and dependencies in
// a build-tool-agnostic format.
package maven

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// MavenProjectInspector implements ProjectInspector for Maven projects.
type MavenProjectInspector struct{}

// NewMavenProjectInspector creates a new Maven project inspector.
func NewMavenProjectInspector() *MavenProjectInspector {
	return &MavenProjectInspector{}
}

// ListModules returns a list of module names declared in the target pom.xml.
func (i *MavenProjectInspector) ListModules(projectRoot string) ([]string, error) {
	pom, err := parsePom(projectRoot)
	if err != nil {
		return nil, err
	}

	return pom.Modules, nil
}

// ListDependencies returns declared dependencies from the pom.xml. Versions are
// resolved against project properties whenever possible so that callers receive
// concrete coordinates instead of raw placeholders.
func (i *MavenProjectInspector) ListDependencies(projectRoot string) ([]core.Dependency, error) {
	pom, err := parsePom(projectRoot)
	if err != nil {
		return nil, err
	}

	return pom.Dependencies, nil
}

// DependencyTree shells out to Maven's dependency:tree goal to obtain the full
// transitive dependency graph. The textual output is parsed into the
// core.DependencyTree structure so the CLI can render it without caring about
// Maven specifics.
func (i *MavenProjectInspector) DependencyTree(projectRoot string) (*core.DependencyTree, error) {
	if _, err := os.Stat(filepath.Join(projectRoot, "pom.xml")); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pom.xml not found at: %s", filepath.Join(projectRoot, "pom.xml"))
		}
		return nil, fmt.Errorf("failed to access pom.xml: %w", err)
	}

	mvnPath, err := exec.LookPath("mvn")
	if err == nil {
		cmd := exec.Command(mvnPath, "-q", "dependency:tree", "-DoutputType=text", "-DappendOutput=false")
		cmd.Dir = projectRoot

		output, execErr := cmd.CombinedOutput()
		if execErr == nil {
			tree, parseErr := parseMavenDependencyTree(string(output))
			if parseErr != nil {
				return nil, parseErr
			}
			return tree, nil
		}

		fallbackTree, fallbackErr := buildDependencyTreeFromPoms(projectRoot)
		if fallbackErr != nil {
			return nil, fmt.Errorf("maven dependency:tree command failed: %w\n%s", execErr, string(output))
		}
		fallbackTree.Warnings = append(fallbackTree.Warnings,
			fmt.Sprintf("maven dependency:tree command failed: %v", execErr),
			"showing best-effort tree from local POM parsing (transitive dependencies may be incomplete)",
		)
		return fallbackTree, nil
	}

	fallbackTree, fallbackErr := buildDependencyTreeFromPoms(projectRoot)
	if fallbackErr != nil {
		return nil, fmt.Errorf("maven executable 'mvn' not found in PATH, and local parsing failed: %w", fallbackErr)
	}
	fallbackTree.Warnings = append(fallbackTree.Warnings,
		"maven executable 'mvn' not found; showing best-effort tree from local POM parsing (transitive dependencies may be incomplete)",
	)
	return fallbackTree, nil
}

// pomDocument is a lightweight in-memory representation of the bits of pom.xml
// we care about for "deps ls" and "module ls".
type pomDocument struct {
	Modules        []string
	Dependencies   []core.Dependency
	Properties     map[string]string
	ProjectVersion string
	ParentVersion  string
	GroupID        string
	ArtifactID     string
	Packaging      string
	ParentGroupID  string
	ParentArtifact string
	ResolvedGroup  string
	ResolvedArt    string
	ResolvedPkg    string
	ResolvedVer    string
}

// parsePom streams the pom.xml file and extracts module names, dependency data,
// and project-level metadata required for placeholder resolution.
func parsePom(projectRoot string) (*pomDocument, error) {
	pomPath := filepath.Join(projectRoot, "pom.xml")

	file, err := os.Open(pomPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pom.xml not found at: %s", pomPath)
		}
		return nil, fmt.Errorf("failed to open pom.xml: %w", err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	decoder.Strict = false

	var (
		doc            pomDocument
		stack          []string
		currentDep     *core.Dependency
		collectModules bool
	)

	doc.Properties = make(map[string]string)

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed parsing pom.xml: %w", err)
		}

		switch tok := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(tok.Name.Local)
			stack = append(stack, name)

			if name == "modules" && hasSuffix(stack, []string{"project", "modules"}) {
				collectModules = true
			}

			if name == "dependency" && isTopLevelDependency(stack) {
				currentDep = &core.Dependency{}
			}

		case xml.EndElement:
			name := strings.ToLower(tok.Name.Local)
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

			if name == "modules" {
				collectModules = false
			}

			if name == "dependency" && currentDep != nil {
				doc.Dependencies = append(doc.Dependencies, *currentDep)
				currentDep = nil
			}

		case xml.CharData:
			data := strings.TrimSpace(string(tok))
			if data == "" {
				continue
			}

			if collectModules && top(stack) == "module" {
				doc.Modules = append(doc.Modules, data)
				continue
			}

			doc.captureProjectMetadata(stack, data)

			if currentDep != nil {
				switch top(stack) {
				case "groupid":
					currentDep.GroupID = data
				case "artifactid":
					currentDep.ArtifactID = data
				case "version":
					currentDep.Version = data
				case "type":
					currentDep.Type = data
				case "scope":
					currentDep.Scope = data
				case "classifier":
					currentDep.Classifier = data
				case "optional":
					currentDep.Optional = strings.EqualFold(data, "true")
				}
			}
		}
	}

	doc.finalize()
	doc.resolveDependencyPlaceholders()

	return &doc, nil
}

// parseMavenDependencyTree converts the textual output of `mvn dependency:tree`
// into a DependencyTree structure. The parser is resilient to Maven's
// informational chatter by ignoring non-tree lines and focusing on the ASCII
// tree markers produced by the plugin.
func parseMavenDependencyTree(raw string) (*core.DependencyTree, error) {
	lines := strings.Split(raw, "\n")

	var (
		root  *core.DependencyNode
		stack []*core.DependencyNode
	)

	for _, line := range lines {
		sanitized, ok := sanitizeMavenTreeLine(line)
		if !ok {
			continue
		}

		depth, coordinate, ok := parseMavenTreeLine(sanitized)
		if !ok {
			continue
		}

		node := &core.DependencyNode{Coordinate: coordinate}

		if depth == 0 {
			root = node
			stack = []*core.DependencyNode{node}
			continue
		}

		if root == nil {
			return nil, fmt.Errorf("failed to locate root node in Maven dependency tree output")
		}

		if depth > len(stack) {
			depth = len(stack)
		}

		parent := stack[depth-1]
		parent.Children = append(parent.Children, node)

		if depth == len(stack) {
			stack = append(stack, node)
		} else {
			stack = append(stack[:depth], node)
		}
	}

	if root == nil {
		return nil, fmt.Errorf("maven dependency tree output did not contain a project root")
	}

	return &core.DependencyTree{Root: root}, nil
}

// sanitizeMavenTreeLine removes Maven log prefixes while preserving the tree
// indentation markers so the parser can reconstruct hierarchy depth.
func sanitizeMavenTreeLine(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n ")
	if line == "" {
		return "", false
	}

	if !strings.HasPrefix(line, "[INFO]") {
		return "", false
	}

	line = strings.TrimPrefix(line, "[INFO]")
	// Preserve leading spaces because they encode indentation depth for the
	// ASCII tree. The dependency:tree goal always emits at least one space after
	// the [INFO] token, so we only trim trailing whitespace.

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(lower, "building") ||
		strings.HasPrefix(lower, "reactor summary") || strings.HasPrefix(lower, "total time") ||
		strings.HasPrefix(lower, "finished at") || strings.HasPrefix(lower, "final memory") ||
		strings.HasPrefix(lower, "scanning for projects") || strings.HasPrefix(lower, "downloading") ||
		strings.HasPrefix(lower, "downloaded") {
		return "", false
	}

	return line, true
}

// parseMavenTreeLine reads a sanitized Maven dependency tree line and returns
// the depth (root = 0) along with the dependency coordinate rendered on that
// line.
func parseMavenTreeLine(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return 0, "", false
	}

	if !strings.Contains(line, "+-") && !strings.Contains(line, "\\-") {
		return 0, trimmed, true
	}

	depth := 0
	i := 0
	for i < len(line) {
		switch {
		case strings.HasPrefix(line[i:], "|  "):
			depth++
			i += 3
		case strings.HasPrefix(line[i:], "   "):
			depth++
			i += 3
		case strings.HasPrefix(line[i:], "+- ") || strings.HasPrefix(line[i:], "\\- "):
			i += 3
			coordinate := strings.TrimSpace(line[i:])
			return depth + 1, coordinate, true
		case strings.HasPrefix(line[i:], "+-") || strings.HasPrefix(line[i:], "\\-"):
			i += 2
			coordinate := strings.TrimSpace(line[i:])
			return depth + 1, coordinate, true
		default:
			i++
		}
	}

	return 0, "", false
}

func buildDependencyTreeFromPoms(projectRoot string) (*core.DependencyTree, error) {
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}

	repoRoot := findRepoRoot(projectRoot)
	cache := make(map[string]*pomDocument)
	index, err := indexPomModules(repoRoot, cache)
	if err != nil {
		return nil, err
	}

	// Ensure the requested project root is indexed even if it lies outside the
	// detected repository root.
	if doc, err := loadPomDocument(projectRoot, cache); err == nil {
		index[moduleCoordinate(doc.ResolvedGroup, doc.ResolvedArt)] = projectRoot
	}

	node, err := buildDependencyTreeNode(projectRoot, cache, index, make(map[string]bool))
	if err != nil {
		return nil, err
	}

	return &core.DependencyTree{Root: node}, nil
}

func findRepoRoot(start string) string {
	dir := start
	for {
		if fileExists(filepath.Join(dir, "go.mod")) || fileExists(filepath.Join(dir, ".git")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

func indexPomModules(root string, cache map[string]*pomDocument) (map[string]string, error) {
	index := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(d.Name()) != "pom.xml" {
			return nil
		}

		dir := filepath.Dir(path)
		doc, parseErr := loadPomDocument(dir, cache)
		if parseErr != nil {
			return nil
		}

		key := moduleCoordinate(doc.ResolvedGroup, doc.ResolvedArt)
		if key == "" {
			return nil
		}

		if _, exists := index[key]; !exists {
			index[key] = dir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return index, nil
}

func buildDependencyTreeNode(dir string, cache map[string]*pomDocument, index map[string]string, visiting map[string]bool) (*core.DependencyNode, error) {
	dir = filepath.Clean(dir)
	if visiting[dir] {
		// Break cycles by returning a node without descending further.
		doc, err := loadPomDocument(dir, cache)
		if err != nil {
			return nil, err
		}
		return &core.DependencyNode{Coordinate: projectCoordinate(doc)}, nil
	}

	doc, err := loadPomDocument(dir, cache)
	if err != nil {
		return nil, err
	}

	visiting[dir] = true
	defer delete(visiting, dir)

	node := &core.DependencyNode{Coordinate: projectCoordinate(doc)}
	for _, dep := range doc.Dependencies {
		child := &core.DependencyNode{Coordinate: dependencyCoordinate(dep)}
		key := moduleCoordinate(dep.GroupID, dep.ArtifactID)
		if key != "" {
			if moduleDir, ok := index[key]; ok {
				subNode, subErr := buildDependencyTreeNode(moduleDir, cache, index, visiting)
				if subErr == nil && subNode != nil {
					child = subNode
				}
			}
		}
		node.Children = append(node.Children, child)
	}

	return node, nil
}

func loadPomDocument(dir string, cache map[string]*pomDocument) (*pomDocument, error) {
	if doc, ok := cache[dir]; ok {
		return doc, nil
	}

	doc, err := parsePom(dir)
	if err != nil {
		return nil, err
	}
	cache[dir] = doc
	return doc, nil
}

func moduleCoordinate(group, artifact string) string {
	group = strings.TrimSpace(group)
	artifact = strings.TrimSpace(artifact)
	if group == "" || artifact == "" || group == "unknown-group" || artifact == "unknown-artifact" {
		return ""
	}
	return group + ":" + artifact
}

func projectCoordinate(doc *pomDocument) string {
	return fmt.Sprintf("%s:%s:%s:%s", doc.ResolvedGroup, doc.ResolvedArt, doc.ResolvedPkg, doc.ResolvedVer)
}

func dependencyCoordinate(dep core.Dependency) string {
	group := strings.TrimSpace(dep.GroupID)
	if group == "" {
		group = "unknown-group"
	}
	artifact := strings.TrimSpace(dep.ArtifactID)
	if artifact == "" {
		artifact = "unknown-artifact"
	}
	packaging := strings.TrimSpace(dep.Type)
	if packaging == "" {
		packaging = "jar"
	}
	version := strings.TrimSpace(dep.Version)
	if version == "" {
		version = "unknown-version"
	}

	coord := fmt.Sprintf("%s:%s:%s:%s", group, artifact, packaging, version)
	if dep.Scope != "" {
		coord = fmt.Sprintf("%s:%s", coord, dep.Scope)
	}
	return coord
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// captureProjectMetadata records project-level information encountered while
// streaming the XML (e.g. versions and custom properties).
func (doc *pomDocument) captureProjectMetadata(stack []string, value string) {
	if hasSuffix(stack, []string{"project", "version"}) {
		doc.ProjectVersion = value
	}

	if hasSuffix(stack, []string{"project", "parent", "version"}) {
		doc.ParentVersion = value
	}

	if hasSuffix(stack, []string{"project", "groupid"}) {
		doc.GroupID = value
	}

	if hasSuffix(stack, []string{"project", "artifactid"}) {
		doc.ArtifactID = value
	}

	if hasSuffix(stack, []string{"project", "packaging"}) {
		doc.Packaging = value
	}

	if hasSuffix(stack, []string{"project", "parent", "groupid"}) {
		doc.ParentGroupID = value
	}

	if hasSuffix(stack, []string{"project", "parent", "artifactid"}) {
		doc.ParentArtifact = value
	}

	if len(stack) >= 3 && stack[len(stack)-3] == "project" && stack[len(stack)-2] == "properties" {
		key := stack[len(stack)-1]
		doc.Properties[key] = value
	}
}

// finalize populates implicit properties such as project.version that may not
// appear in the <properties> block but are required for placeholder resolution.
func (doc *pomDocument) finalize() {
	if doc.Properties == nil {
		doc.Properties = make(map[string]string)
	}

	if doc.ProjectVersion != "" {
		doc.Properties["project.version"] = doc.ProjectVersion
	} else if doc.ParentVersion != "" {
		doc.Properties["project.version"] = doc.ParentVersion
	}

	if doc.GroupID != "" {
		doc.ResolvedGroup = doc.GroupID
	} else if doc.ParentGroupID != "" {
		doc.ResolvedGroup = doc.ParentGroupID
	}

	if doc.ArtifactID != "" {
		doc.ResolvedArt = doc.ArtifactID
	} else if doc.ParentArtifact != "" {
		doc.ResolvedArt = doc.ParentArtifact
	}

	if doc.ProjectVersion != "" {
		doc.ResolvedVer = doc.ProjectVersion
	} else if doc.ParentVersion != "" {
		doc.ResolvedVer = doc.ParentVersion
	}

	if doc.Packaging != "" {
		doc.ResolvedPkg = doc.Packaging
	} else {
		doc.ResolvedPkg = "jar"
	}

	if doc.ResolvedGroup == "" {
		doc.ResolvedGroup = "unknown-group"
	}

	if doc.ResolvedArt == "" {
		doc.ResolvedArt = "unknown-artifact"
	}

	if doc.ResolvedVer == "" {
		doc.ResolvedVer = "unknown-version"
	}
}

// placeholderPattern matches Maven-style property placeholders such as
// "${project.version}" or "${spring.boot.version}".
var placeholderPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// resolveDependencyPlaceholders applies property interpolation to each captured
// dependency so consumers receive normalized values.
func (doc *pomDocument) resolveDependencyPlaceholders() {
	for i := range doc.Dependencies {
		doc.Dependencies[i].Version = doc.resolvePlaceholders(doc.Dependencies[i].Version)
		doc.Dependencies[i].Scope = doc.resolvePlaceholders(doc.Dependencies[i].Scope)
		doc.Dependencies[i].Type = doc.resolvePlaceholders(doc.Dependencies[i].Type)
		doc.Dependencies[i].Classifier = doc.resolvePlaceholders(doc.Dependencies[i].Classifier)
	}
}

// resolvePlaceholders replaces all property placeholders within a single value
// with their resolved form if available.
func (doc *pomDocument) resolvePlaceholders(value string) string {
	if value == "" {
		return value
	}

	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		key := match[2 : len(match)-1]
		if resolved, ok := doc.Properties[key]; ok {
			return resolved
		}
		return match
	})
}

// top returns the last element in the stack helper slice.
func top(stack []string) string {
	if len(stack) == 0 {
		return ""
	}
	return stack[len(stack)-1]
}

// hasSuffix checks whether the parsing stack ends with the provided sequence.
func hasSuffix(stack, suffix []string) bool {
	if len(stack) < len(suffix) {
		return false
	}
	start := len(stack) - len(suffix)
	for i, name := range suffix {
		if stack[start+i] != name {
			return false
		}
	}
	return true
}

// isTopLevelDependency verifies that the current stack location corresponds to
// a <dependency> element directly under <project><dependencies>.
func isTopLevelDependency(stack []string) bool {
	if len(stack) < 3 {
		return false
	}
	// Expect ... -> project -> dependencies -> dependency
	return stack[len(stack)-3] == "project" && stack[len(stack)-2] == "dependencies" && stack[len(stack)-1] == "dependency"
}
