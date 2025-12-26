package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/native"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [path]",
	Short: "Build the project (JPM engine)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		m, _, err := core.LoadManifest(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no jpm.yaml found in %s (run 'jpm init')", abs)
			}
			return err
		}

		// Check if native engine is requested
		if strings.EqualFold(strings.TrimSpace(m.Engine), "native") {
			return buildWithNativeEngine(abs, m)
		}

		jpmDir := filepath.Join(abs, ".jpm")
		mvnDir := filepath.Join(jpmDir, "maven")
		if err := os.MkdirAll(filepath.Join(mvnDir, "src", "main"), 0o755); err != nil {
			return err
		}
		// Ensure symlink: .jpm/maven/src/main/java -> ./src
		if err := ensureJavaLink(abs, mvnDir); err != nil {
			return err
		}
		// Render pom.xml from manifest
		pomPath := filepath.Join(mvnDir, "pom.xml")
		pom := renderPomFromManifest(m)
		if err := os.WriteFile(pomPath, []byte(pom), 0o644); err != nil {
			return err
		}

		outDir := filepath.Join(jpmDir, "out")
		logsDir := filepath.Join(jpmDir, "logs")
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(logsDir, 0o755); err != nil {
			return err
		}
		artifactID := m.Project.ArtifactID
		if artifactID == "" {
			artifactID = "app"
		}
		version := m.Project.Version
		if version == "" {
			version = "0.1.0-SNAPSHOT"
		}
		outJar := filepath.Join(outDir, fmt.Sprintf("%s-%s.jar", artifactID, version))

		// Hidden engine: prefer mvn if available, else write placeholder
		depJars, err := ensureDependencyJars(abs, m.Dependencies)
		if err != nil {
			return err
		}
		classpathFile := filepath.Join(outDir, "classpath")
		if len(depJars) == 0 {
			_ = os.Remove(classpathFile)
		} else if err := os.WriteFile(classpathFile, []byte(strings.Join(depJars, string(os.PathListSeparator))), 0o644); err != nil {
			return err
		}

		if _, err := exec.LookPath("mvn"); err == nil {
			logFile := filepath.Join(logsDir, fmt.Sprintf("build-%d.log", time.Now().Unix()))
			if err := runMavenPackage(mvnDir, logFile); err != nil {
				return err
			}
			// Copy artifact from target to .jpm/out
			srcJar := filepath.Join(mvnDir, "target", fmt.Sprintf("%s-%s.jar", artifactID, version))
			if err := copyFile(srcJar, outJar); err != nil {
				return err
			}
		} else {
			if err := fallbackBuildWithoutMaven(abs, logsDir, outJar, depJars); err != nil {
				return err
			}
		}

		fmt.Println(headerStyle("→ build:"))
		fmt.Printf("  %s %s\n", primaryTextStyle("artifact"), outJar)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

// buildWithNativeEngine uses the native JPM engine for the build.
func buildWithNativeEngine(projectRoot string, m *core.Manifest) error {
	eng := native.New()
	result, err := eng.Build(projectRoot, m)
	if err != nil {
		if result != nil && result.Logs != "" {
			fmt.Fprintln(os.Stderr, result.Logs)
		}
		return err
	}

	fmt.Println(headerStyle("→ build (native engine):"))
	fmt.Printf("  %s %s\n", primaryTextStyle("artifact"), result.ArtifactPath)
	if len(result.ClasspathJars) > 0 {
		fmt.Printf("  %s %d dependencies resolved\n", primaryTextStyle("deps"), len(result.ClasspathJars))
	}
	return nil
}

func ensureJavaLink(projectRoot, mvnDir string) error {
	link := filepath.Join(mvnDir, "src", "main", "java")
	src := filepath.Join(projectRoot, "src")
	_ = os.Remove(link)
	if err := os.Symlink(src, link); err != nil {
		// Windows or restricted FS: fallback to creating the dir
		if runtime.GOOS == "windows" || strings.Contains(err.Error(), "operation not permitted") {
			return os.MkdirAll(link, 0o755)
		}
		return err
	}
	return nil
}

func renderPomFromManifest(m *core.Manifest) string {
	java := m.Java.Version
	if java == "" {
		java = "21"
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(b, "<project xmlns=\"http://maven.apache.org/POM/4.0.0\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xsi:schemaLocation=\"http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd\">\n")
	fmt.Fprintf(b, "  <modelVersion>4.0.0</modelVersion>\n")
	fmt.Fprintf(b, "  <groupId>%s</groupId>\n", m.Project.GroupID)
	fmt.Fprintf(b, "  <artifactId>%s</artifactId>\n", m.Project.ArtifactID)
	fmt.Fprintf(b, "  <version>%s</version>\n", m.Project.Version)
	fmt.Fprintf(b, "  <properties>\n")
	fmt.Fprintf(b, "    <maven.compiler.source>%s</maven.compiler.source>\n", java)
	fmt.Fprintf(b, "    <maven.compiler.target>%s</maven.compiler.target>\n", java)
	fmt.Fprintf(b, "  </properties>\n")
	fmt.Fprintf(b, "  <dependencies>\n")
	for _, d := range m.Dependencies {
		if d.GroupID == "" || d.ArtifactID == "" || d.Version == "" {
			continue
		}
		fmt.Fprintf(b, "    <dependency>\n")
		fmt.Fprintf(b, "      <groupId>%s</groupId>\n", d.GroupID)
		fmt.Fprintf(b, "      <artifactId>%s</artifactId>\n", d.ArtifactID)
		fmt.Fprintf(b, "      <version>%s</version>\n", d.Version)
		if d.Scope != "" {
			fmt.Fprintf(b, "      <scope>%s</scope>\n", d.Scope)
		}
		if d.Type != "" {
			fmt.Fprintf(b, "      <type>%s</type>\n", d.Type)
		}
		if d.Classifier != "" {
			fmt.Fprintf(b, "      <classifier>%s</classifier>\n", d.Classifier)
		}
		if d.Optional {
			fmt.Fprintf(b, "      <optional>true</optional>\n")
		}
		fmt.Fprintf(b, "    </dependency>\n")
	}
	fmt.Fprintf(b, "  </dependencies>\n")
	fmt.Fprintf(b, "</project>\n")
	return b.String()
}

func runMavenPackage(mvnDir, logPath string) error {
	cmd := exec.Command("mvn", "-q", "-f", filepath.Join(mvnDir, "pom.xml"), "package")
	cmd.Dir = mvnDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	_ = os.WriteFile(logPath, buf.Bytes(), 0o644)
	if err != nil {
		return fmt.Errorf("maven build failed (see %s)", logPath)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func fallbackBuildWithoutMaven(projectRoot, logsDir, outJar string, depJars []string) error {
	javacPath, err := exec.LookPath("javac")
	if err != nil {
		return fmt.Errorf("javac not found on PATH (install JDK or Maven)")
	}

	srcDir := filepath.Join(projectRoot, "src")
	javaFiles, err := collectJavaFiles(srcDir)
	if err != nil {
		return err
	}
	if len(javaFiles) == 0 {
		return fmt.Errorf("no Java sources found under %s", srcDir)
	}

	classesDir := filepath.Join(projectRoot, ".jpm", "tmp", "classes")
	if err := os.RemoveAll(classesDir); err != nil {
		return err
	}
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		return err
	}

	javacLog := filepath.Join(logsDir, fmt.Sprintf("build-javac-%d.log", time.Now().Unix()))
	var compileBuf bytes.Buffer
	args := []string{"-d", classesDir}
	if len(depJars) > 0 {
		args = append(args, "-classpath", strings.Join(depJars, string(os.PathListSeparator)))
	}
	args = append(args, javaFiles...)
	compileCmd := exec.Command(javacPath, args...)
	compileCmd.Stdout = &compileBuf
	compileCmd.Stderr = &compileBuf
	if err := compileCmd.Run(); err != nil {
		_ = os.WriteFile(javacLog, compileBuf.Bytes(), 0o644)
		return fmt.Errorf("javac compilation failed (see %s)", javacLog)
	}
	if compileBuf.Len() > 0 {
		_ = os.WriteFile(javacLog, compileBuf.Bytes(), 0o644)
	}

	jarPath, err := exec.LookPath("jar")
	if err != nil {
		return fmt.Errorf("jar tool not found on PATH; install a full JDK")
	}
	jarLog := filepath.Join(logsDir, fmt.Sprintf("build-jar-%d.log", time.Now().Unix()))
	var jarBuf bytes.Buffer
	jarCmd := exec.Command(jarPath, "cf", outJar, "-C", classesDir, ".")
	jarCmd.Stdout = &jarBuf
	jarCmd.Stderr = &jarBuf
	if err := jarCmd.Run(); err != nil {
		_ = os.WriteFile(jarLog, jarBuf.Bytes(), 0o644)
		return fmt.Errorf("jar packaging failed (see %s)", jarLog)
	}
	if jarBuf.Len() > 0 {
		_ = os.WriteFile(jarLog, jarBuf.Bytes(), 0o644)
	}

	return nil
}

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

func ensureDependencyJars(projectRoot string, deps []core.Dependency) ([]string, error) {
	cacheRoot := filepath.Join(projectRoot, ".jpm", "cache")
	var jars []string
	for _, dep := range deps {
		group := strings.TrimSpace(dep.GroupID)
		artifact := strings.TrimSpace(dep.ArtifactID)
		version := strings.TrimSpace(dep.Version)
		if group == "" || artifact == "" || version == "" {
			continue
		}
		packaging := strings.TrimSpace(dep.Type)
		if packaging == "" {
			packaging = "jar"
		}
		if packaging != "jar" {
			continue
		}

		classifier := strings.TrimSpace(dep.Classifier)
		fileName := artifact + "-" + version
		if classifier != "" {
			fileName += "-" + classifier
		}
		fileName += "." + packaging

		groupPath := strings.ReplaceAll(group, ".", "/")
		destDir := filepath.Join(cacheRoot, groupPath, artifact, version)
		destPath := filepath.Join(destDir, fileName)

		if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
			jars = append(jars, destPath)
			continue
		}

		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return nil, err
		}

		url := fmt.Sprintf("https://repo1.maven.org/maven2/%s/%s/%s/%s", groupPath, artifact, version, fileName)
		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", url, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to download %s: %s", url, resp.Status)
		}
		tmpPath := destPath + ".tmp"
		out, err := os.Create(tmpPath)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		if _, err := io.Copy(out, resp.Body); err != nil {
			out.Close()
			resp.Body.Close()
			_ = os.Remove(tmpPath)
			return nil, err
		}
		out.Close()
		resp.Body.Close()
		if err := os.Rename(tmpPath, destPath); err != nil {
			return nil, err
		}

		jars = append(jars, destPath)
	}
	return jars, nil
}
