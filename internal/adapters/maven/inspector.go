// Package maven contains the ProjectInspector implementation for Apache Maven
// projects. It parses pom.xml files to surface module lists and dependencies in
// a build-tool-agnostic format.
package maven

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
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

// pomDocument is a lightweight in-memory representation of the bits of pom.xml
// we care about for "deps show" and "module find".
type pomDocument struct {
	Modules        []string
	Dependencies   []core.Dependency
	Properties     map[string]string
	ProjectVersion string
	ParentVersion  string
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

// captureProjectMetadata records project-level information encountered while
// streaming the XML (e.g. versions and custom properties).
func (doc *pomDocument) captureProjectMetadata(stack []string, value string) {
	if hasSuffix(stack, []string{"project", "version"}) {
		doc.ProjectVersion = value
	}

	if hasSuffix(stack, []string{"project", "parent", "version"}) {
		doc.ParentVersion = value
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
