package core

import "fmt"

// BuildTool represents the supported build tools
type BuildTool int

const (
	Maven BuildTool = iota
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

// ParseBuildTool parses a string into a BuildTool
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