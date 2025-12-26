package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFetcher_RepositoryURL tests URL construction.
func TestFetcher_RepositoryURL(t *testing.T) {
	f := NewFetcher("", false)

	tests := []struct {
		group    string
		artifact string
		version  string
		ext      string
		want     string
	}{
		{
			"org.slf4j", "slf4j-api", "2.0.16", ".pom",
			"https://repo1.maven.org/maven2/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.pom",
		},
		{
			"com.google.guava", "guava", "33.0.0", ".jar",
			"https://repo1.maven.org/maven2/com/google/guava/guava/33.0.0/guava-33.0.0.jar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.artifact, func(t *testing.T) {
			got := f.repositoryURL(f.repos[0], tt.group, tt.artifact, tt.version, "", tt.ext)
			if got != tt.want {
				t.Errorf("repositoryURL() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestFetcher_CachePathForArtifact tests cache path generation.
func TestFetcher_CachePathForArtifact(t *testing.T) {
	tmpdir := t.TempDir()
	f := NewFetcher(tmpdir, false)

	path := f.cachePathForArtifact("org.slf4j", "slf4j-api", "2.0.16", "", "pom", ".pom")

	// Verify structure: tmpdir/pom/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.pom
	if !filepath.HasPrefix(path, tmpdir) {
		t.Errorf("Path not under cache dir: %s", path)
	}

	expectedSuffix := "slf4j-api-2.0.16.pom"
	if !hasSuffix(path, expectedSuffix) {
		t.Errorf("Path has wrong suffix: %s (expected to end with %s)", path, expectedSuffix)
	}
}

// TestFetcher_ComputeSHA256 tests checksum computation.
func TestFetcher_ComputeSHA256(t *testing.T) {
	data := []byte("hello world")
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	got := computeSHA256(data)
	if got != expected {
		t.Errorf("computeSHA256() = %s, want %s", got, expected)
	}
}

// TestFetcher_CacheArtifact tests artifact caching.
func TestFetcher_CacheArtifact(t *testing.T) {
	tmpdir := t.TempDir()
	f := NewFetcher(tmpdir, false)

	path := filepath.Join(tmpdir, "test", "artifact.jar")
	data := []byte("test artifact data")

	if err := f.cacheArtifact(path, data); err != nil {
		t.Fatalf("cacheArtifact failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Cached file not found: %v", err)
	}

	// Verify contents
	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(read) != string(data) {
		t.Errorf("Cached data mismatch")
	}
}

// TestFetcher_FetchPOMFromCache tests cached fetch.
func TestFetcher_FetchPOMFromCache(t *testing.T) {
	tmpdir := t.TempDir()
	f := NewFetcher(tmpdir, false)

	// Pre-cache a POM
	pomData := []byte(`<?xml version="1.0"?><project></project>`)
	gav := "org.example:app:1.0"
	parts := parseGAV(gav)
	cachePath := f.cachePathForArtifact(parts[0], parts[1], parts[2], "", "pom", ".pom")

	if err := f.cacheArtifact(cachePath, pomData); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Fetch (should come from cache)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := f.FetchPOM(ctx, gav)
	if err != nil {
		t.Fatalf("FetchPOM failed: %v", err)
	}

	if string(data) != string(pomData) {
		t.Errorf("Data mismatch: got %d bytes, want %d", len(data), len(pomData))
	}
}

// TestFetcher_FetchPOMFromServer tests fetching from mock server.
func TestFetcher_FetchPOMFromServer(t *testing.T) {
	tmpdir := t.TempDir()

	// Setup mock server
	pomContent := []byte(`<?xml version="1.0"?><project><groupId>org.example</groupId></project>`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/org/example/app/1.0/app-1.0.pom" {
			w.Write(pomContent)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create fetcher with mock server
	f := NewFetcher(tmpdir, false)
	f.repos = []string{server.URL}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := f.FetchPOM(ctx, "org.example:app:1.0")
	if err != nil {
		t.Fatalf("FetchPOM failed: %v", err)
	}

	if string(data) != string(pomContent) {
		t.Errorf("Data mismatch")
	}

	// Verify it was cached
	parts := parseGAV("org.example:app:1.0")
	cachePath := f.cachePathForArtifact(parts[0], parts[1], parts[2], "", "pom", ".pom")
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("Cached file not found: %v", err)
	}
}

// TestFetcher_CacheStats tests cache statistics.
func TestFetcher_CacheStats(t *testing.T) {
	tmpdir := t.TempDir()
	f := NewFetcher(tmpdir, false)

	// Add some cached artifacts
	data1 := []byte("artifact 1")
	data2 := []byte("artifact 2 content")

	path1 := filepath.Join(tmpdir, "pom", "org", "app", "1.0", "app-1.0.pom")
	path2 := filepath.Join(tmpdir, "artifacts", "com", "lib", "2.0", "lib-2.0.jar")

	if err := f.cacheArtifact(path1, data1); err != nil {
		t.Fatalf("cacheArtifact failed: %v", err)
	}
	if err := f.cacheArtifact(path2, data2); err != nil {
		t.Fatalf("cacheArtifact failed: %v", err)
	}

	stats, err := f.CacheStats()
	if err != nil {
		t.Fatalf("CacheStats failed: %v", err)
	}

	if stats.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", stats.FileCount)
	}

	expectedSize := int64(len(data1) + len(data2))
	if stats.TotalSize != expectedSize {
		t.Errorf("TotalSize = %d, want %d", stats.TotalSize, expectedSize)
	}
}

// TestParallelFetcher_FetchPOMsParallel tests parallel POM fetching.
func TestParallelFetcher_FetchPOMsParallel(t *testing.T) {
	tmpdir := t.TempDir()

	// Setup mock server with multiple POMs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch path {
		case "/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.pom":
			w.Write([]byte(`<project><groupId>org.slf4j</groupId></project>`))
		case "/com/google/guava/guava/33.0.0/guava-33.0.0.pom":
			w.Write([]byte(`<project><groupId>com.google</groupId></project>`))
		case "/junit/junit/4.13/junit-4.13.pom":
			w.Write([]byte(`<project><groupId>junit</groupId></project>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := NewFetcher(tmpdir, false)
	f.repos = []string{server.URL}

	pf := NewParallelFetcher(f, 2, false)

	gavs := []string{
		"org.slf4j:slf4j-api:2.0.16",
		"com.google.guava:guava:33.0.0",
		"junit:junit:4.13",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := pf.FetchPOMsParallel(ctx, gavs)
	if err != nil {
		t.Fatalf("FetchPOMsParallel failed: %v", err)
	}

	if len(data) != 3 {
		t.Errorf("Expected 3 POMs, got %d", len(data))
	}

	for _, gav := range gavs {
		if _, ok := data[gav]; !ok {
			t.Errorf("Missing POM for %s", gav)
		}
	}
}

// TestFetcher_Retry tests retry logic on failure.
func TestFetcher_Retry(t *testing.T) {
	tmpdir := t.TempDir()

	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 2 {
			// Fail first attempt
			http.Error(w, "Server error", http.StatusInternalServerError)
		} else {
			// Succeed on retry
			w.Write([]byte("pom content"))
		}
	}))
	defer server.Close()

	f := NewFetcher(tmpdir, false)
	f.repos = []string{server.URL}
	f.SetMaxRetries(3)
	f.SetRetryDelay(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := f.FetchPOM(ctx, "org.example:app:1.0")
	if err != nil {
		t.Fatalf("FetchPOM failed: %v", err)
	}

	if attemptCount < 2 {
		t.Errorf("Retry not attempted: attemptCount = %d", attemptCount)
	}

	if string(data) != "pom content" {
		t.Errorf("Data mismatch")
	}
}

// TestFetcher_ContextCancellation tests context cancellation.
func TestFetcher_ContextCancellation(t *testing.T) {
	tmpdir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("pom content"))
	}))
	defer server.Close()

	f := NewFetcher(tmpdir, false)
	f.repos = []string{server.URL}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := f.FetchPOM(ctx, "org.example:app:1.0")
	if err == nil {
		t.Errorf("Expected context deadline error, got nil")
	}
}

// BenchmarkFetcher_FetchFromCache benchmarks cache hit performance.
func BenchmarkFetcher_FetchFromCache(b *testing.B) {
	tmpdir := b.TempDir()
	f := NewFetcher(tmpdir, false)

	// Pre-cache a POM
	pomData := []byte(`<?xml version="1.0"?><project></project>`)
	parts := parseGAV("org.example:app:1.0")
	cachePath := f.cachePathForArtifact(parts[0], parts[1], parts[2], "", "pom", ".pom")
	f.cacheArtifact(cachePath, pomData)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FetchPOM(ctx, "org.example:app:1.0")
	}
}

// hasSuffix checks if path ends with suffix (helper for filepath)
func hasSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}

