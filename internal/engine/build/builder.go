package build

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/KhalidEchchahid/go-jpm/internal/engine/compiler"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/packager"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/resolver"
)

// BuildConfig holds configuration for the build process.
type BuildConfig struct {
	ProjectName string
	Version     string
	MainClass   string
	JavaVersion string
	Verbose     bool
	ToolVersion string
}

// BuildResult contains the output of a complete build.
type BuildResult struct {
	CompileResult *compiler.CompileResult
	PackageResult *packager.PackageResult
	Duration      time.Duration
	Success       bool
	Errors        []string
}

// Builder orchestrates compilation and packaging.
type Builder struct {
	config   *BuildConfig
	compiler *compiler.Compiler
	packager *packager.Packager
}

// NewBuilder creates a new builder instance.
func NewBuilder(config *BuildConfig) *Builder {
	return &Builder{
		config:   config,
		compiler: compiler.NewCompiler(config.Verbose, config.ToolVersion),
		packager: packager.NewPackager(config.Verbose, config.ToolVersion),
	}
}

// Build performs a complete build: resolve dependencies, compile, and package.
func (b *Builder) Build(
	ctx context.Context,
	srcDir, workDir, outputDir string,
	graph *resolver.Graph,
) (*BuildResult, error) {
	start := time.Now()
	result := &BuildResult{
		Errors: []string{},
	}

	// Step 1: Compile
	classesDir := filepath.Join(workDir, "classes")
	classpath, err := graph.ToClasspath()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to build classpath: %v", err))
		result.Duration = time.Since(start)
		return result, err
	}

	if b.config.Verbose {
		fmt.Printf("Compiling with classpath: %v\n", classpath)
	}

	compileResult, err := b.compiler.Compile(ctx, srcDir, classesDir, classpath, b.config.JavaVersion)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Compilation failed: %v", err))
		result.Duration = time.Since(start)
		return result, err
	}
	result.CompileResult = compileResult

	if b.config.Verbose {
		fmt.Printf("Compiled %d sources to %d classes\n", compileResult.Sources, compileResult.Classes)
	}

	// Step 2: Package
	jarPath, err := b.packager.Package(
		ctx,
		classesDir,
		outputDir,
		b.config.ProjectName,
		b.config.Version,
		b.config.MainClass,
	)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Packaging failed: %v", err))
		result.Duration = time.Since(start)
		return result, err
	}

	packageResult, _ := b.packager.PackageWithResult(
		ctx,
		classesDir,
		outputDir,
		b.config.ProjectName,
		b.config.Version,
		b.config.MainClass,
	)
	result.PackageResult = packageResult

	if b.config.Verbose {
		fmt.Printf("Packaged to: %s (%d bytes)\n", jarPath, packageResult.Size)
	}

	result.Success = true
	result.Duration = time.Since(start)
	return result, nil
}

// CompileOnly performs only compilation without packaging.
func (b *Builder) CompileOnly(
	ctx context.Context,
	srcDir, classesDir string,
	classpath []string,
) (*compiler.CompileResult, error) {
	return b.compiler.Compile(ctx, srcDir, classesDir, classpath, b.config.JavaVersion)
}

// PackageOnly performs only packaging given existing compiled classes.
func (b *Builder) PackageOnly(
	ctx context.Context,
	classesDir, outputDir string,
) (*packager.PackageResult, error) {
	return b.packager.PackageWithResult(ctx, classesDir, outputDir, b.config.ProjectName, b.config.Version, b.config.MainClass)
}
