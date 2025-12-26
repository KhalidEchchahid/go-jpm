package native

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Compiler handles Java source compilation.
type Compiler struct {
	javaVersion string
	classpath   []string
	srcDir      string
	outDir      string
}

// NewCompiler creates a compiler targeting the given output directory.
func NewCompiler(srcDir, outDir, javaVersion string) *Compiler {
	return &Compiler{
		javaVersion: javaVersion,
		srcDir:      srcDir,
		outDir:      outDir,
	}
}

// SetClasspath sets the compilation classpath.
func (c *Compiler) SetClasspath(jars []string) {
	c.classpath = jars
}

// Compile compiles all Java sources under srcDir to outDir.
// Returns the compilation log output and any error.
func (c *Compiler) Compile() (string, error) {
	javacPath, err := exec.LookPath("javac")
	if err != nil {
		return "", fmt.Errorf("javac not found on PATH (install JDK)")
	}

	javaFiles, err := collectJavaFiles(c.srcDir)
	if err != nil {
		return "", err
	}
	if len(javaFiles) == 0 {
		return "", fmt.Errorf("no Java sources found under %s", c.srcDir)
	}

	// Clean and recreate output directory
	if err := os.RemoveAll(c.outDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(c.outDir, 0o755); err != nil {
		return "", err
	}

	// Build arguments
	args := []string{"-d", c.outDir}

	// Add source/target version if specified
	if c.javaVersion != "" {
		args = append(args, "-source", c.javaVersion, "-target", c.javaVersion)
	}

	// Add classpath
	if len(c.classpath) > 0 {
		args = append(args, "-classpath", strings.Join(c.classpath, string(os.PathListSeparator)))
	}

	args = append(args, javaFiles...)

	var buf bytes.Buffer
	cmd := exec.Command(javacPath, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("javac compilation failed: %w", err)
	}

	return buf.String(), nil
}

// collectJavaFiles walks a directory tree and returns all .java file paths.
func collectJavaFiles(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".java") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
