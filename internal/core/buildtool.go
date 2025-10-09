package core

import "fmt"

// BuildTool represents the supported build tools
type BuildTool int

const (
	// Maven designates Apache Maven projects that rely on pom.xml descriptors.
	Maven BuildTool = iota
	// Gradle designates Gradle projects. The Gradle adapter is currently a stub
	// but is kept in the enum so the CLI can validate user intent consistently.
	Gradle
)

// String returns the string representation of the build tool
func (bt BuildTool) String() string {
	switch bt {
	case Maven:
		return "maven"
	case Gradle:
		return "gradle"
	default:
		return "unknown"
	}
}

// ParseBuildTool parses a string into a BuildTool. It returns a typed value plus
// an error describing unsupported entries so callers can surface helpful CLI
// feedback.
func ParseBuildTool(s string) (BuildTool, error) {
	switch s {
	case "maven":
		return Maven, nil
	case "gradle":
		return Gradle, nil
	default:
		return Maven, fmt.Errorf("unsupported build tool: %s", s)
	}
}
