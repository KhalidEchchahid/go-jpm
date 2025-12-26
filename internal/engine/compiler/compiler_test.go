package compiler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCompile_SimpleClass(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	outDir := t.TempDir()

	// Write a simple Java file
	sourceCode := `
public class Hello {
    public static void main(String[] args) {
        System.out.println("Hello, World!");
    }
}
`
	javaFile := filepath.Join(srcDir, "Hello.java")
	if err := os.WriteFile(javaFile, []byte(sourceCode), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Compile
	compiler := NewCompiler(false, "0.0.354")
	result, err := compiler.Compile(context.Background(), srcDir, outDir, []string{}, "17")

	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if result.Sources != 1 {
		t.Errorf("Expected 1 source, got %d", result.Sources)
	}

	if result.Classes != 1 {
		t.Errorf("Expected 1 class, got %d", result.Classes)
	}

	// Verify Hello.class exists
	classFile := filepath.Join(outDir, "Hello.class")
	if _, err := os.Stat(classFile); err != nil {
		t.Errorf("Hello.class not found: %v", err)
	}
}

func TestCompile_NoSources(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	compiler := NewCompiler(false, "0.0.354")
	result, err := compiler.Compile(context.Background(), srcDir, outDir, []string{}, "17")

	if err != nil {
		t.Fatalf("Compilation should succeed with no sources: %v", err)
	}

	if result.Sources != 0 {
		t.Errorf("Expected 0 sources, got %d", result.Sources)
	}

	if result.Classes != 0 {
		t.Errorf("Expected 0 classes, got %d", result.Classes)
	}
}

func TestCompile_MultipleSources(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	// Write multiple Java files
	files := map[string]string{
		"Hello.java": `public class Hello { public static void greet() {} }`,
		"World.java": `public class World { public static void main(String[] args) {} }`,
	}

	for name, code := range files {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(code), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	compiler := NewCompiler(false, "0.0.354")
	result, err := compiler.Compile(context.Background(), srcDir, outDir, []string{}, "17")

	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if result.Sources != 2 {
		t.Errorf("Expected 2 sources, got %d", result.Sources)
	}

	if result.Classes != 2 {
		t.Errorf("Expected 2 classes, got %d", result.Classes)
	}
}

func TestCompile_InvalidJavaVersion(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	compiler := NewCompiler(false, "0.0.354")
	_, err := compiler.Compile(context.Background(), srcDir, outDir, []string{}, "99")

	if err == nil {
		t.Error("Expected error for invalid Java version")
	}
}

func TestCompile_ValidJavaVersions(t *testing.T) {
	versions := []string{"17", "21", "23"}

	for _, version := range versions {
		srcDir := t.TempDir()
		outDir := t.TempDir()

		// Write a simple Java file
		if err := os.WriteFile(filepath.Join(srcDir, "Test.java"), []byte("public class Test {}"), 0644); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}

		compiler := NewCompiler(false, "0.0.354")
		_, err := compiler.Compile(context.Background(), srcDir, outDir, []string{}, version)

		if err != nil {
			t.Errorf("Compilation with Java %s failed: %v", version, err)
		}
	}
}

func TestDiscoverSources_Sorted(t *testing.T) {
	srcDir := t.TempDir()

	// Create files in non-alphabetical order
	files := []string{"Zebra.java", "Alpha.java", "Beta.java"}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(srcDir, file), []byte("public class Test {}"), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", file, err)
		}
	}

	sources, err := discoverSources(srcDir)
	if err != nil {
		t.Fatalf("Failed to discover sources: %v", err)
	}

	if len(sources) != 3 {
		t.Errorf("Expected 3 sources, got %d", len(sources))
	}

	// Verify sorted order
	expected := []string{"Alpha.java", "Beta.java", "Zebra.java"}
	for i, exp := range expected {
		if !filepath.HasPrefix(sources[i], filepath.Join(srcDir, exp)) {
			t.Errorf("Expected %s at position %d, got %s", exp, i, filepath.Base(sources[i]))
		}
	}
}

func TestBuildClasspath(t *testing.T) {
	tests := []struct {
		name       string
		classpath  []string
		outDir     string
		shouldHave string
	}{
		{
			name:       "empty",
			classpath:  []string{},
			outDir:     "",
			shouldHave: "",
		},
		{
			name:       "outdir_only",
			classpath:  []string{},
			outDir:     "/out",
			shouldHave: "/out",
		},
		{
			name:       "with_artifacts",
			classpath:  []string{"/lib/a.jar", "/lib/b.jar"},
			outDir:     "/out",
			shouldHave: "/out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildClasspath(tt.classpath, tt.outDir)
			if tt.shouldHave != "" && !contains(result, tt.shouldHave) {
				t.Errorf("Expected classpath to contain %s, got %s", tt.shouldHave, result)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && (s == substr || s[0:len(substr)] == substr)
}

func TestParseCompilerOutput(t *testing.T) {
	output := `
Test.java:5: warning: [deprecation] method() has been deprecated
Test.java:10: error: cannot find symbol
`

	messages := parseCompilerOutput(output)
	if len(messages) < 2 {
		t.Errorf("Expected at least 2 messages, got %d", len(messages))
	}
}

func TestValidateJavaVersion(t *testing.T) {
	valid := []string{"17", "21", "23"}
	for _, v := range valid {
		if err := validateJavaVersion(v); err != nil {
			t.Errorf("Version %s should be valid: %v", v, err)
		}
	}

	invalid := []string{"11", "8", "99", ""}
	for _, v := range invalid {
		if err := validateJavaVersion(v); err == nil {
			t.Errorf("Version %s should be invalid", v)
		}
	}
}
