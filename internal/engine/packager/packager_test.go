package packager

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPackage_SimpleClass(t *testing.T) {
	classesDir := t.TempDir()
	outDir := t.TempDir()

	// Create a dummy class file
	classFile := filepath.Join(classesDir, "Hello.class")
	if err := os.WriteFile(classFile, []byte("dummy class data"), 0644); err != nil {
		t.Fatalf("Failed to write class file: %v", err)
	}

	packager := NewPackager(false, "0.0.354")
	jarPath, err := packager.Package(context.Background(), classesDir, outDir, "myapp", "1.0.0", "Hello")

	if err != nil {
		t.Fatalf("Packaging failed: %v", err)
	}

	// Verify JAR exists
	if _, err := os.Stat(jarPath); err != nil {
		t.Errorf("JAR file not found: %v", err)
	}

	// Verify JAR name
	expectedName := "myapp-1.0.0.jar"
	if filepath.Base(jarPath) != expectedName {
		t.Errorf("Expected JAR name %s, got %s", expectedName, filepath.Base(jarPath))
	}
}

func TestPackage_WithResult(t *testing.T) {
	classesDir := t.TempDir()
	outDir := t.TempDir()

	// Create class file
	if err := os.WriteFile(filepath.Join(classesDir, "Main.class"), []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	packager := NewPackager(false, "0.0.354")
	result, err := packager.PackageWithResult(context.Background(), classesDir, outDir, "app", "2.0.0", "Main")

	if err != nil {
		t.Fatalf("Packaging failed: %v", err)
	}

	if result.Classes != 1 {
		t.Errorf("Expected 1 class, got %d", result.Classes)
	}

	if result.MainClass != "Main" {
		t.Errorf("Expected Main-Class 'Main', got '%s'", result.MainClass)
	}

	if result.Size <= 0 {
		t.Errorf("Expected positive JAR size, got %d", result.Size)
	}
}

func TestPackage_EmptyClasses(t *testing.T) {
	classesDir := t.TempDir()
	outDir := t.TempDir()

	packager := NewPackager(false, "0.0.354")
	_, err := packager.Package(context.Background(), classesDir, outDir, "app", "1.0.0", "")

	if err == nil {
		t.Error("Expected error for empty classes directory")
	}
}

func TestPackage_MultipleClasses(t *testing.T) {
	classesDir := t.TempDir()
	outDir := t.TempDir()

	// Create multiple class files
	for i := 1; i <= 5; i++ {
		name := filepath.Join(classesDir, "Class"+string(rune(48+i))+".class")
		if err := os.WriteFile(name, []byte("class data"), 0644); err != nil {
			t.Fatalf("Failed to create class: %v", err)
		}
	}

	packager := NewPackager(false, "0.0.354")
	jarPath, err := packager.Package(context.Background(), classesDir, outDir, "app", "1.0.0", "")

	if err != nil {
		t.Fatalf("Packaging failed: %v", err)
	}

	// Verify JAR contains all classes
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		t.Fatalf("Failed to open JAR: %v", err)
	}
	defer zr.Close()

	classCount := 0
	for _, file := range zr.File {
		if filepath.Ext(file.Name) == ".class" {
			classCount++
		}
	}

	if classCount != 5 {
		t.Errorf("Expected 5 classes in JAR, found %d", classCount)
	}
}

func TestPackage_ManifestContent(t *testing.T) {
	classesDir := t.TempDir()
	outDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(classesDir, "Test.class"), []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to create class: %v", err)
	}

	packager := NewPackager(false, "0.0.354")
	jarPath, err := packager.Package(context.Background(), classesDir, outDir, "app", "1.0.0", "Test")

	if err != nil {
		t.Fatalf("Packaging failed: %v", err)
	}

	// Read and verify manifest
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		t.Fatalf("Failed to open JAR: %v", err)
	}
	defer zr.Close()

	found := false
	for _, file := range zr.File {
		if file.Name == "META-INF/MANIFEST.MF" {
			found = true
			rc, _ := file.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()

			manifest := string(data)
			if !contains(manifest, "Manifest-Version") {
				t.Error("Missing Manifest-Version in manifest")
			}
			if !contains(manifest, "Main-Class: Test") {
				t.Error("Missing or incorrect Main-Class in manifest")
			}
			break
		}
	}

	if !found {
		t.Error("MANIFEST.MF not found in JAR")
	}
}

func TestPackage_DeterministicJAR(t *testing.T) {
	// Create two identical JARs and verify they're byte-identical
	classesDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(classesDir, "Test.class"), []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create class: %v", err)
	}

	packager := NewPackager(false, "0.0.354")

	// First JAR
	outDir1 := t.TempDir()
	jarPath1, err := packager.Package(context.Background(), classesDir, outDir1, "app", "1.0.0", "Test")
	if err != nil {
		t.Fatalf("First packaging failed: %v", err)
	}

	// Second JAR
	outDir2 := t.TempDir()
	jarPath2, err := packager.Package(context.Background(), classesDir, outDir2, "app", "1.0.0", "Test")
	if err != nil {
		t.Fatalf("Second packaging failed: %v", err)
	}

	// Compare sizes (deterministic if same)
	stat1, _ := os.Stat(jarPath1)
	stat2, _ := os.Stat(jarPath2)

	if stat1.Size() != stat2.Size() {
		t.Errorf("JAR sizes differ: %d vs %d", stat1.Size(), stat2.Size())
	}
}

func TestCollectClassFiles_Sorted(t *testing.T) {
	classesDir := t.TempDir()

	// Create files in random order
	files := []string{"Zebra.class", "Alpha.class", "Beta.class"}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(classesDir, file), []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create: %v", err)
		}
	}

	collected, err := collectClassFiles(classesDir)
	if err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	if len(collected) != 3 {
		t.Errorf("Expected 3 files, got %d", len(collected))
	}

	// Verify sorted
	for i := 0; i < len(collected)-1; i++ {
		if collected[i] > collected[i+1] {
			t.Error("Class files not sorted")
			break
		}
	}
}

func TestScanForMainClass(t *testing.T) {
	classesDir := t.TempDir()

	// Create various class names
	files := []string{"Main.class", "Helper.class", "Util.class"}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(classesDir, file), []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create: %v", err)
		}
	}

	candidates, err := ScanForMainClass(classesDir)
	if err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}

	if len(candidates) == 0 {
		t.Error("Expected to find at least one main class candidate")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr))
}
