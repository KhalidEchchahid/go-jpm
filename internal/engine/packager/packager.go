package packager

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PackageResult represents the output of packaging.
type PackageResult struct {
	JAR        string        `json:"jar"`
	Size       int64         `json:"size"`
	Classes    int           `json:"classes"`
	MainClass  string        `json:"main_class,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// Packager creates JAR archives from compiled classes.
type Packager struct {
	verbose bool
	version string
}

// NewPackager creates a new packager instance.
func NewPackager(verbose bool, version string) *Packager {
	return &Packager{
		verbose: verbose,
		version: version,
	}
}

// Package creates a JAR file from the compiled classes directory.
func (p *Packager) Package(
	ctx context.Context,
	classesDir, outDir, artifact, pkgVersion, mainClass string,
) (string, error) {
	start := time.Now()

	// Validate input
	if classesDir == "" {
		return "", fmt.Errorf("classesDir cannot be empty")
	}
	if outDir == "" {
		return "", fmt.Errorf("outDir cannot be empty")
	}

	// Check if classesDir exists and has content
	entries, err := os.ReadDir(classesDir)
	if err != nil {
		return "", fmt.Errorf("reading classesDir: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no classes to package in %s", classesDir)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	// Build JAR filename
	jarName := fmt.Sprintf("%s-%s.jar", artifact, pkgVersion)
	jarPath := filepath.Join(outDir, jarName)

	// Create JAR file
	jarFile, err := os.Create(jarPath)
	if err != nil {
		return "", fmt.Errorf("creating JAR file: %w", err)
	}
	defer jarFile.Close()

	// Create ZIP writer
	zipWriter := zip.NewWriter(jarFile)
	defer zipWriter.Close()

	// Collect all class files (sorted for determinism)
	classFiles, err := collectClassFiles(classesDir)
	if err != nil {
		return "", fmt.Errorf("collecting classes: %w", err)
	}

	// Write manifest
	if err := writeManifest(zipWriter, mainClass, pkgVersion, p.version); err != nil {
		return "", fmt.Errorf("writing manifest: %w", err)
	}

	// Write class files to ZIP
	classCount := 0
	for _, classFile := range classFiles {
		if err := addFileToZip(zipWriter, classFile, classesDir); err != nil {
			return "", fmt.Errorf("adding file to ZIP: %w", err)
		}
		classCount++
	}

	if p.verbose {
		fmt.Printf("Created JAR: %s (%d classes) in %v\n", jarPath, classCount, time.Since(start))
	}

	// Get JAR size
	stat, err := os.Stat(jarPath)
	if err != nil {
		return "", fmt.Errorf("getting JAR size: %w", err)
	}

	if p.verbose {
		fmt.Printf("JAR size: %d bytes\n", stat.Size())
	}

	return jarPath, nil
}

// PackageWithResult creates a JAR and returns detailed result.
func (p *Packager) PackageWithResult(
	ctx context.Context,
	classesDir, outDir, artifact, pkgVersion, mainClass string,
) (*PackageResult, error) {
	start := time.Now()

	jarPath, err := p.Package(ctx, classesDir, outDir, artifact, pkgVersion, mainClass)
	if err != nil {
		return nil, err
	}

	// Count classes
	classFiles, _ := collectClassFiles(classesDir)

	// Get JAR size
	stat, _ := os.Stat(jarPath)

	return &PackageResult{
		JAR:       jarPath,
		Size:      stat.Size(),
		Classes:   len(classFiles),
		MainClass: mainClass,
		Duration:  time.Since(start),
	}, nil
}

// collectClassFiles returns all .class files in classesDir, sorted.
func collectClassFiles(classesDir string) ([]string, error) {
	var classFiles []string

	err := filepath.WalkDir(classesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".class") {
			classFiles = append(classFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(classFiles)
	return classFiles, nil
}

// writeManifest writes the META-INF/MANIFEST.MF file.
func writeManifest(zw *zip.Writer, mainClass, version, toolVersion string) error {
	manifest := "Manifest-Version: 1.0\r\n"
	manifest += fmt.Sprintf("Created-By: JPM %s\r\n", toolVersion)
	manifest += fmt.Sprintf("Implementation-Version: %s\r\n", version)
	if mainClass != "" {
		manifest += fmt.Sprintf("Main-Class: %s\r\n", mainClass)
	}

	// Create manifest header
	header := &zip.FileHeader{
		Name:     "META-INF/MANIFEST.MF",
		Modified: time.Unix(0, 0),
	}
	header.SetMode(0644)

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(manifest))
	return err
}

// addFileToZip adds a file to the ZIP archive with deterministic metadata.
func addFileToZip(zw *zip.Writer, filePath, baseDir string) error {
	// Get relative path for ZIP entry
	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return err
	}

	// Normalize to forward slashes for ZIP
	zipEntryName := filepath.ToSlash(relPath)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Create header with deterministic metadata
	header := &zip.FileHeader{
		Name:     zipEntryName,
		Modified: time.Unix(0, 0), // Epoch for reproducibility
	}
	header.SetMode(0644)

	// Create entry and write data
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// ScanForMainClass scans classesDir for classes with main method.
func ScanForMainClass(classesDir string) ([]string, error) {
	classFiles, err := collectClassFiles(classesDir)
	if err != nil {
		return nil, err
	}

	var mainClasses []string
	for _, classFile := range classFiles {
		// Convert file path to class name
		relPath, _ := filepath.Rel(classesDir, classFile)
		className := strings.TrimSuffix(filepath.ToSlash(relPath), ".class")
		className = strings.ReplaceAll(className, "/", ".")

		// For now, collect all candidates; actual scanning would use bytecode analysis
		// As a simple heuristic, include classes in default package or main packages
		if strings.Contains(className, "Main") || !strings.Contains(className, ".") {
			mainClasses = append(mainClasses, className)
		}
	}

	return mainClasses, nil
}

// String returns a JSON representation of PackageResult.
func (pr *PackageResult) String() string {
	data, _ := json.MarshalIndent(pr, "", "  ")
	return string(data)
}
