package fetcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheLayout defines the directory structure for cached artifacts.
const (
	CacheDir     = ".jpm/cache"        // Root cache directory
	POMDir       = "pom"               // Subdirectory for POM files
	ArtifactDir  = "artifacts"         // Subdirectory for JAR/AAR files
	ChecksumFile = ".checksum"         // Filename for SHA-256 checksums
)

// Fetcher retrieves artifacts from Maven Central.
type Fetcher struct {
	cacheDir       string              // Root cache directory path
	repos          []string            // Repository URLs to try
	client         *http.Client        // HTTP client with timeouts
	checksumCache  map[string]string   // artifact GAV -> SHA-256
	mu             sync.RWMutex        // Protect caches
	maxRetries     int                 // Max retry attempts
	retryDelay     time.Duration       // Delay between retries
	verbose        bool
}

// ChecksumInfo stores integrity information for an artifact.
type ChecksumInfo struct {
	SHA256    string
	Algorithm string
	Generated time.Time
}

// FetchResult contains metadata about a fetched artifact.
type FetchResult struct {
	LocalPath string        // Path to cached artifact
	URL       string        // URL it was fetched from
	Cached    bool          // true if from cache, false if freshly downloaded
	Checksum  string        // SHA-256 of file
	Size      int64         // File size in bytes
	Duration  time.Duration // Download time (0 if cached)
}

// NewFetcher creates a new fetcher with Maven Central as default repo.
func NewFetcher(cacheDir string, verbose bool) *Fetcher {
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, CacheDir)
	}

	return &Fetcher{
		cacheDir:      cacheDir,
		repos:         []string{"https://repo1.maven.org/maven2"},
		checksumCache: make(map[string]string),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
				DisableCompression:  false,
				MaxIdleConnsPerHost: 5,
			},
		},
		maxRetries: 3,
		retryDelay: 1 * time.Second,
		verbose:    verbose,
	}
}

// FetchPOM retrieves a POM file from cache or repository.
// gav format: "group:artifact:version" → "group/artifact/version/artifact-version.pom"
func (f *Fetcher) FetchPOM(ctx context.Context, gav string) ([]byte, error) {
	return f.fetch(ctx, gav, "pom", ".pom")
}

// FetchArtifact retrieves a JAR or other artifact.
// gav format: "group:artifact:version" → "group/artifact/version/artifact-version.jar"
func (f *Fetcher) FetchArtifact(ctx context.Context, gav string, classifier string) ([]byte, error) {
	ext := ".jar"
	if classifier != "" {
		gav = fmt.Sprintf("%s:%s", gav, classifier)
	}
	return f.fetch(ctx, gav, "artifacts", ext)
}

// fetch is the internal method for fetching any artifact.
func (f *Fetcher) fetch(ctx context.Context, gav string, subdir string, ext string) ([]byte, error) {
	// Parse GAV
	parts := parseGAV(gav)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid GAV: %s", gav)
	}

	groupID := parts[0]
	artifactID := parts[1]
	version := parts[2]
	classifier := ""
	if len(parts) > 3 {
		classifier = parts[3]
	}

	// Generate local path
	localPath := f.cachePathForArtifact(groupID, artifactID, version, classifier, subdir, ext)

	// Check cache first
	if data, err := os.ReadFile(localPath); err == nil {
		if f.verbose {
			fmt.Printf("✓ Cache hit: %s\n", gav)
		}
		return data, nil
	}

	// Not in cache; fetch from repository
	if f.verbose {
		fmt.Printf("→ Fetching: %s\n", gav)
	}

	var data []byte
	var err error
	var url string

	for attempt := 1; attempt <= f.maxRetries; attempt++ {
		for _, repo := range f.repos {
			repoURL := f.repositoryURL(repo, groupID, artifactID, version, classifier, ext)
			url = repoURL

			if f.verbose {
				fmt.Printf("  Try %d: %s\n", attempt, repoURL)
			}

			data, err = f.fetchFromURL(ctx, repoURL)
			if err == nil {
				break
			}
		}

		if err == nil {
			break
		}

		if attempt < f.maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(f.retryDelay):
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("fetch failed after %d attempts: %w", f.maxRetries, err)
	}

	// Verify checksum (if available)
	if err := f.verifyChecksum(ctx, url, data); err != nil && f.verbose {
		fmt.Printf("⚠ Checksum verification skipped: %v\n", err)
	}

	// Cache the artifact
	if err := f.cacheArtifact(localPath, data); err != nil {
		return nil, fmt.Errorf("cache write failed: %w", err)
	}

	if f.verbose {
		fmt.Printf("✓ Cached: %s (%d bytes)\n", gav, len(data))
	}

	return data, nil
}

// fetchFromURL retrieves data from an HTTP URL with retry logic.
func (f *Fetcher) fetchFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// cachePathForArtifact generates the local cache path for an artifact.
func (f *Fetcher) cachePathForArtifact(groupID, artifactID, version, classifier, subdir, ext string) string {
	// group/artifact/version/artifact[-classifier]-version.ext
	filename := fmt.Sprintf("%s-%s%s%s", artifactID, version, classifierSuffix(classifier), ext)
	groupPath := filepath.Join(strings.Split(groupID, ".")...)
	return filepath.Join(f.cacheDir, subdir, groupPath, artifactID, version, filename)
}

// repositoryURL constructs a Maven Central URL for an artifact.
// Format: repo/group/artifact/version/artifact-version.ext
func (f *Fetcher) repositoryURL(repo, groupID, artifactID, version, classifier, ext string) string {
	groupPath := filepath.ToSlash(filepath.Join(strings.Split(groupID, ".")...))
	filename := fmt.Sprintf("%s-%s%s%s", artifactID, version, classifierSuffix(classifier), ext)
	return fmt.Sprintf("%s/%s/%s/%s/%s", repo, groupPath, artifactID, version, filename)
}

// cacheArtifact writes an artifact to the cache directory.
func (f *Fetcher) cacheArtifact(path string, data []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write file
	return os.WriteFile(path, data, 0644)
}

// verifyChecksum attempts to fetch and verify SHA-256.
func (f *Fetcher) verifyChecksum(ctx context.Context, url string, data []byte) error {
	// Try to fetch .sha256 file
	checksumURL := url + ".sha256"
	checksumData, err := f.fetchFromURL(ctx, checksumURL)
	if err != nil {
		return err
	}

	// Parse checksum (format: "abc123def456..." or "abc123def456... filename")
	checksumStr := string(checksumData)
	fields := strings.Fields(checksumStr)
	if len(fields) == 0 {
		return fmt.Errorf("invalid checksum format")
	}
	expectedChecksum := fields[0]

	// Verify
	actualChecksum := computeSHA256(data)
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// computeSHA256 calculates SHA-256 hash of data.
func computeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:])
}

// CachePath returns the root cache directory.
func (f *Fetcher) CachePath() string {
	return f.cacheDir
}

// ClearCache removes all cached artifacts.
func (f *Fetcher) ClearCache() error {
	return os.RemoveAll(f.cacheDir)
}

// CacheStats returns cache statistics.
func (f *Fetcher) CacheStats() (CacheStatistics, error) {
	stats := CacheStatistics{
		Dir:        f.cacheDir,
		Timestamp:  time.Now(),
		ArtifactIDs: make(map[string]int64),
	}

	err := filepath.Walk(f.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			stats.FileCount++
			stats.TotalSize += info.Size()

			// Extract artifact ID from path
			rel, _ := filepath.Rel(f.cacheDir, path)
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) >= 3 {
				artifactID := parts[1]
				stats.ArtifactIDs[artifactID] += info.Size()
			}
		}
		return nil
	})

	return stats, err
}

// CacheStatistics holds cache statistics.
type CacheStatistics struct {
	Dir         string
	FileCount   int64
	TotalSize   int64
	Timestamp   time.Time
	ArtifactIDs map[string]int64 // artifactID -> total size
}

// parseGAV parses "group:artifact:version[:classifier]"
func parseGAV(gav string) []string {
	return strings.Split(gav, ":")
}

// classifierSuffix returns "-classifier" or empty string.
func classifierSuffix(classifier string) string {
	if classifier == "" {
		return ""
	}
	return "-" + classifier
}

// AddRepository adds an additional repository to search.
func (f *Fetcher) AddRepository(url string) {
	f.repos = append(f.repos, url)
}

// SetMaxRetries sets the maximum retry attempts.
func (f *Fetcher) SetMaxRetries(maxRetries int) {
	f.maxRetries = maxRetries
}

// SetRetryDelay sets the delay between retries.
func (f *Fetcher) SetRetryDelay(delay time.Duration) {
	f.retryDelay = delay
}
