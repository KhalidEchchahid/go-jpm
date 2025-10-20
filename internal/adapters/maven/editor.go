package maven

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
)

// AddDependencyOptions controls how dependency mutations are applied to a
// Maven project.
type AddDependencyOptions struct {
	DryRun bool
}

// AddDependencyResult describes the outcome of mutating a pom.xml file.
type AddDependencyResult struct {
	Path       string
	Added      bool
	Updated    bool
	DryRun     bool
	Before     string
	After      string
	Coordinate string
}

// AddDependency inserts or updates a dependency entry inside the target
// project's pom.xml. When DryRun is enabled, the file is left untouched and the
// resulting XML is surfaced via the result.
func AddDependency(projectRoot string, dep core.Dependency, opts AddDependencyOptions) (*AddDependencyResult, error) {
	pomPath := filepath.Join(projectRoot, "pom.xml")
	beforeBytes, err := os.ReadFile(pomPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pom.xml: %w", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(beforeBytes); err != nil {
		return nil, fmt.Errorf("failed to parse pom.xml: %w", err)
	}

	projectElem := doc.SelectElement("project")
	if projectElem == nil {
		return nil, fmt.Errorf("pom.xml is missing <project> root element")
	}

	dependenciesElem := firstChildElement(projectElem, "dependencies")
	if dependenciesElem == nil {
		dependenciesElem = etree.NewElement("dependencies")
		projectElem.AddChild(dependenciesElem)
	}

	result := &AddDependencyResult{
		Path:       pomPath,
		Coordinate: dependencyCoordinate(dep),
		DryRun:     opts.DryRun,
		Before:     string(beforeBytes),
	}

	if existing := findDependencyElement(dependenciesElem, dep.GroupID, dep.ArtifactID); existing != nil {
		changed := updateDependencyElement(existing, dep)
		result.Updated = changed
		if !changed {
			result.After = result.Before
			return result, nil
		}
	} else {
		dependenciesElem.AddChild(newDependencyElement(dep))
		result.Added = true
	}

	doc.Indent(2)
	var buffer bytes.Buffer
	if _, err := doc.WriteTo(&buffer); err != nil {
		return nil, fmt.Errorf("failed to serialize pom.xml: %w", err)
	}
	result.After = buffer.String()

	if opts.DryRun {
		return result, nil
	}

	if err := os.WriteFile(pomPath, buffer.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write pom.xml: %w", err)
	}

	return result, nil
}

func newDependencyElement(dep core.Dependency) *etree.Element {
	elem := etree.NewElement("dependency")
	appendChildWithText(elem, "groupId", dep.GroupID)
	appendChildWithText(elem, "artifactId", dep.ArtifactID)
	appendChildWithText(elem, "version", dep.Version)
	appendChildWithText(elem, "type", dep.Type)
	appendChildWithText(elem, "scope", dep.Scope)
	appendChildWithText(elem, "classifier", dep.Classifier)
	if dep.Optional {
		appendChildWithText(elem, "optional", "true")
	}
	return elem
}

func updateDependencyElement(elem *etree.Element, dep core.Dependency) bool {
	changed := false
	changed = setChildText(elem, "version", dep.Version) || changed
	changed = setChildText(elem, "type", dep.Type) || changed
	changed = setChildText(elem, "scope", dep.Scope) || changed
	changed = setChildText(elem, "classifier", dep.Classifier) || changed
	optionalValue := ""
	if dep.Optional {
		optionalValue = "true"
	}
	changed = setChildText(elem, "optional", optionalValue) || changed
	return changed
}

func findDependencyElement(parent *etree.Element, groupID, artifactID string) *etree.Element {
	groupID = strings.TrimSpace(groupID)
	artifactID = strings.TrimSpace(artifactID)
	for _, child := range parent.ChildElements() {
		if strings.EqualFold(child.Tag, "dependency") {
			if matchDependency(child, groupID, artifactID) {
				return child
			}
		}
	}
	return nil
}

func matchDependency(elem *etree.Element, groupID, artifactID string) bool {
	grp := childText(elem, "groupId")
	art := childText(elem, "artifactId")
	return strings.EqualFold(grp, groupID) && strings.EqualFold(art, artifactID)
}

func childText(elem *etree.Element, tag string) string {
	child := elem.FindElement(tag)
	if child == nil {
		return ""
	}
	return strings.TrimSpace(child.Text())
}

func setChildText(elem *etree.Element, tag, value string) bool {
	value = strings.TrimSpace(value)
	child := elem.FindElement(tag)
	if value == "" {
		if child != nil {
			elem.RemoveChild(child)
			return true
		}
		return false
	}

	if child == nil {
		child = etree.NewElement(tag)
		elem.AddChild(child)
	}

	if strings.TrimSpace(child.Text()) == value {
		return false
	}

	child.SetText(value)
	return true
}

func appendChildWithText(parent *etree.Element, tag, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	child := etree.NewElement(tag)
	child.SetText(value)
	parent.AddChild(child)
}

func firstChildElement(parent *etree.Element, tag string) *etree.Element {
	for _, child := range parent.ChildElements() {
		if strings.EqualFold(child.Tag, tag) {
			return child
		}
	}
	return nil
}
