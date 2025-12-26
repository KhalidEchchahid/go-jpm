package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/build"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/fetcher"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/lockfile"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/resolver"
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

		// Get verbose flag
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			return err
		}

		// Route to appropriate build engine
		if m.Engine == "native" {
			return buildNative(abs, m, verbose)
		}

		// Fall back to Maven (default)
		return buildMaven(abs, m)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
}

// buildMaven uses the Maven bridge to compile and package the project.
func buildMaven(projectRoot string, m *core.Manifest) error {
	jpmDir := filepath.Join(projectRoot, ".jpm")
	mvnDir := filepath.Join(jpmDir, "maven")
	if err := os.MkdirAll(filepath.Join(mvnDir, "src", "main"), 0o755); err != nil {
		return err
	}
	// Ensure symlink: .jpm/maven/src/main/java -> ./src
	if err := ensureJavaLink(projectRoot, mvnDir); err != nil {
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
		// Placeholder artifact when mvn is not available
		if _, err := os.Stat(outJar); err != nil && os.IsNotExist(err) {
			if err := os.WriteFile(outJar, []byte{}, 0o644); err != nil {
				return err
			}
		}
	}

	fmt.Println(headerStyle("→ build:"))
	fmt.Printf("  %s %s\n", primaryTextStyle("artifact"), outJar)
	return nil
}

// buildNative uses the native engine (resolver → compiler → packager) to build.
func buildNative(projectRoot string, m *core.Manifest, verbose bool) error {
	ctx := context.Background()

	// Setup directories
	jpmDir := filepath.Join(projectRoot, ".jpm")
	outDir := filepath.Join(jpmDir, "out")
	workDir := filepath.Join(jpmDir, "work")

	if verbose {
		fmt.Printf("  %s %s\n", subduedStyle("setup"), "project directories")
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}

	// Get artifact info from manifest
	artifactID := m.Project.ArtifactID
	if artifactID == "" {
		artifactID = "app"
	}
	version := m.Project.Version
	if version == "" {
		version = "0.1.0-SNAPSHOT"
	}
	javaVersion := m.Java.Version
	if javaVersion == "" {
		javaVersion = "21"
	}
	mainClass := m.App.MainClass

	fmt.Println(headerStyle("→ building (native engine)"))
	if verbose {
		fmt.Printf("  %s artifact: %s version: %s java: %s main: %s\n",
			subduedStyle("manifest"),
			artifactID, version, javaVersion, mainClass)
	}

	// Create builder
	config := &build.BuildConfig{
		ProjectName: artifactID,
		Version:     version,
		MainClass:   mainClass,
		JavaVersion: javaVersion,
		Verbose:     verbose,
		ToolVersion: "0.0.1", // TODO: Get actual version from binary
	}
	builder := build.NewBuilder(config)
	if verbose {
		fmt.Printf("  %s builder initialized\n", subduedStyle("build"))
	}

	// Create fetcher
	cacheDir := filepath.Join(jpmDir, "cache")
	f := fetcher.NewFetcher(cacheDir, verbose)
	if verbose {
		fmt.Printf("  %s fetcher ready (cache: %s)\n", subduedStyle("resolve"), cacheDir)
	}

	// Create resolver
	depResolver := resolver.NewResolver(f, verbose, "0.0.1")
	if verbose {
		fmt.Printf("  %s resolver created\n", subduedStyle("resolve"))
	}

	// Convert manifest dependencies to resolver format
	rootDeps := make([]resolver.Dependency, 0, len(m.Dependencies))
	for _, dep := range m.Dependencies {
		rootDeps = append(rootDeps, resolver.Dependency{
			GroupID:    dep.GroupID,
			ArtifactID: dep.ArtifactID,
			Version:    dep.Version,
			Scope:      dep.Scope,
			Type:       dep.Type,
			Classifier: dep.Classifier,
			Optional:   dep.Optional,
		})
	}

	// Resolve dependencies (including transitive)
	var graph *resolver.Graph
	var err error
	if len(rootDeps) > 0 {
		if verbose {
			fmt.Printf("  %s resolving %d root dependencies (transitive included)\n", subduedStyle("resolve"), len(rootDeps))
		}
		graph, err = depResolver.Resolve(ctx, rootDeps)
		if err != nil {
			return fmt.Errorf("dependency resolution failed: %w", err)
		}

		// Set cache path on graph for classpath generation
		graph.CachePath = cacheDir

		// Fetch JAR files for all resolved dependencies
		if verbose {
			fmt.Printf("  %s fetching %d JAR files\n", subduedStyle("resolve"), len(graph.Nodes))
		}
		// Build list of artifact GAVs. Classifiers are uncommon; for now we fetch
		// the base JARs in parallel and fall back to sequential for classifier cases.
		gavs := make([]string, 0, len(graph.Nodes))
		classifierNodes := make([]*resolver.Node, 0)
		for _, node := range graph.Nodes {
			if node == nil || !node.Resolved {
				continue
			}
			if node.Classifier != "" {
				classifierNodes = append(classifierNodes, node)
				continue
			}
			gavs = append(gavs, fmt.Sprintf("%s:%s:%s", node.GroupID, node.ArtifactID, node.Version))
		}

		// Parallel fetch for base artifacts.
		if len(gavs) > 0 {
			pf := fetcher.NewParallelFetcher(f, 8, verbose)
			if _, err := pf.FetchArtifactsParallel(ctx, gavs); err != nil {
				// Don't fail the build; fall back to sequential fetch with warnings.
				if verbose {
					fmt.Printf("  ⚠ parallel fetch failed; falling back to sequential: %v\n", err)
				}
				for _, gav := range gavs {
					_, ferr := f.FetchArtifact(ctx, gav, "")
					if ferr != nil && verbose {
						fmt.Printf("  ⚠ Failed to fetch JAR %s: %v\n", gav, ferr)
					}
				}
			}
		}

		// Sequential fetch for classifier artifacts (kept separate for now).
		for _, node := range classifierNodes {
			gav := fmt.Sprintf("%s:%s:%s", node.GroupID, node.ArtifactID, node.Version)
			_, err := f.FetchArtifact(ctx, gav, node.Classifier)
			if err != nil && verbose {
				fmt.Printf("  ⚠ Failed to fetch JAR %s:%s: %v\n", gav, node.Classifier, err)
			}
		}
	} else {
		graph = resolver.NewGraph("0.0.1")
		graph.CachePath = cacheDir
		if verbose {
			fmt.Printf("  %s no dependencies to resolve\n", subduedStyle("resolve"))
		}
	}

	// Build
	srcDir := filepath.Join(projectRoot, "src")
	if verbose {
		fmt.Printf("  %s source from %s\n", subduedStyle("compile"), srcDir)
	}
	result, err := builder.Build(ctx, srcDir, workDir, outDir, graph)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return fmt.Errorf("build errors: %v", result.Errors)
		}
		return fmt.Errorf("build failed")
	}

	// Generate lockfile
	if verbose {
		fmt.Printf("  %s writing lockfile to %s\n", subduedStyle("package"), filepath.Join(projectRoot, "jpm.lock.yaml"))
	}
	lockWriter := lockfile.NewWriter("0.0.1")
	if err := lockWriter.Write(projectRoot, graph, "native"); err != nil {
		// Warn but don't fail on lockfile generation errors
		fmt.Printf("  %s: %v\n", primaryTextStyle("warning"), err)
	}

	// Report success
	jarPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.jar", artifactID, version))
	fmt.Printf("  %s %s\n", primaryTextStyle("compiled"), fmt.Sprintf("%d source files", result.CompileResult.Sources))
	fmt.Printf("  %s %s\n", primaryTextStyle("packaged"), jarPath)
	fmt.Printf("  %s %v\n", primaryTextStyle("duration"), result.Duration)

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

// parseGAV parses "group:artifact:version[:classifier]" into parts
func parseGAV(gav string) []string {
	return strings.Split(gav, ":")
}
