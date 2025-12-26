package native

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// Packager creates JAR archives from compiled classes.
type Packager struct {
	classesDir string
	outputJar  string
	mainClass  string // optional, for manifest Main-Class
}

// NewPackager creates a packager for the given classes directory and output JAR path.
func NewPackager(classesDir, outputJar string) *Packager {
	return &Packager{
		classesDir: classesDir,
		outputJar:  outputJar,
	}
}

// SetMainClass sets the Main-Class for the JAR manifest.
func (p *Packager) SetMainClass(mainClass string) {
	p.mainClass = mainClass
}

// Package creates a JAR file from the compiled classes.
// Returns the packaging log output and any error.
func (p *Packager) Package() (string, error) {
	jarPath, err := exec.LookPath("jar")
	if err != nil {
		return "", fmt.Errorf("jar tool not found on PATH; install a full JDK")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(parent(p.outputJar), 0o755); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	var cmd *exec.Cmd

	if p.mainClass != "" {
		// Create with manifest entry
		cmd = exec.Command(jarPath, "cfe", p.outputJar, p.mainClass, "-C", p.classesDir, ".")
	} else {
		cmd = exec.Command(jarPath, "cf", p.outputJar, "-C", p.classesDir, ".")
	}

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("jar packaging failed: %w", err)
	}

	return buf.String(), nil
}

// parent returns the parent directory of a path.
func parent(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
