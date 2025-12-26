package native

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

const (
	// MavenCentralBase is the default repository URL.
	MavenCentralBase = "https://repo1.maven.org/maven2"
)

// Downloader fetches artifacts from remote Maven repositories.
type Downloader struct {
	baseURL string
	client  *http.Client
	cache   *Cache
	mu      sync.Mutex
}

// NewDownloader creates a downloader targeting the given base URL and cache.
func NewDownloader(baseURL string, cache *Cache) *Downloader {
	if baseURL == "" {
		baseURL = MavenCentralBase
	}
	return &Downloader{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{},
		cache:   cache,
	}
}

// Artifact represents a Maven coordinate.
type Artifact struct {
	GroupID    string
	ArtifactID string
	Version    string
	Classifier string
	Packaging  string // defaults to "jar"
}

// Coordinate returns a string representation of the artifact.
func (a Artifact) Coordinate() string {
	coord := fmt.Sprintf("%s:%s:%s", a.GroupID, a.ArtifactID, a.Version)
	if a.Classifier != "" {
		coord += ":" + a.Classifier
	}
	return coord
}

// URL returns the remote URL for the artifact.
func (d *Downloader) URL(a Artifact) string {
	groupPath := strings.ReplaceAll(a.GroupID, ".", "/")
	packaging := a.Packaging
	if packaging == "" {
		packaging = "jar"
	}
	fileName := a.ArtifactID + "-" + a.Version
	if a.Classifier != "" {
		fileName += "-" + a.Classifier
	}
	fileName += "." + packaging
	return fmt.Sprintf("%s/%s/%s/%s/%s", d.baseURL, groupPath, a.ArtifactID, a.Version, fileName)
}

// POMURL returns the remote URL for an artifact's POM.
func (d *Downloader) POMURL(a Artifact) string {
	groupPath := strings.ReplaceAll(a.GroupID, ".", "/")
	fileName := fmt.Sprintf("%s-%s.pom", a.ArtifactID, a.Version)
	return fmt.Sprintf("%s/%s/%s/%s/%s", d.baseURL, groupPath, a.ArtifactID, a.Version, fileName)
}

// EnsureArtifact downloads the artifact JAR if not already cached.
// Returns the local path to the artifact.
func (d *Downloader) EnsureArtifact(a Artifact) (string, error) {
	packaging := a.Packaging
	if packaging == "" {
		packaging = "jar"
	}
	localPath := d.cache.ArtifactPath(a.GroupID, a.ArtifactID, a.Version, a.Classifier, packaging)
	if d.cache.Has(localPath) {
		return localPath, nil
	}
	url := d.URL(a)
	if err := d.download(url, localPath); err != nil {
		return "", fmt.Errorf("download artifact %s: %w", a.Coordinate(), err)
	}
	return localPath, nil
}

// EnsurePOM downloads the POM file if not already cached.
// Returns the local path to the POM.
func (d *Downloader) EnsurePOM(a Artifact) (string, error) {
	localPath := d.cache.POMPath(a.GroupID, a.ArtifactID, a.Version)
	if d.cache.Has(localPath) {
		return localPath, nil
	}
	url := d.POMURL(a)
	if err := d.download(url, localPath); err != nil {
		return "", fmt.Errorf("download POM %s: %w", a.Coordinate(), err)
	}
	return localPath, nil
}

// download fetches a URL and writes it to destPath atomically.
func (d *Downloader) download(url, destPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Double-check after acquiring lock
	if d.cache.Has(destPath) {
		return nil
	}

	if err := d.cache.EnsureDir(destPath); err != nil {
		return err
	}

	resp, err := d.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s for %s", resp.Status, url)
	}

	tmpPath := destPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}

	return os.Rename(tmpPath, destPath)
}
