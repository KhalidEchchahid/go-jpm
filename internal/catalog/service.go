package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	cacheFileName     = "maven_catalog.json"
	cacheVersion      = 1
	cacheTTL          = 24 * time.Hour
	maxSuggestions    = 20
	mavenSearchURL    = "https://search.maven.org/solrsearch/select"
	mavenDefaultRows  = 100
	httpClientTimeout = 10 * time.Second
)

// Service surfaces Maven artifact metadata and suggestions backed by a small
// on-disk cache. Results are refreshed from Maven Central when stale.
type Service struct {
	client *http.Client
	cache  *cache
	clock  func() time.Time
}

// ArtifactSuggestion describes a dependency coordinate suggestion suitable for
// shell completion or interactive prompts.
type ArtifactSuggestion struct {
	GroupID       string
	ArtifactID    string
	LatestVersion string
	Description   string
}

// ArtifactMetadata represents a fully hydrated dependency including the list of
// known versions. Versions are sorted from newest to oldest.
type ArtifactMetadata struct {
	GroupID       string
	ArtifactID    string
	LatestVersion string
	Versions      []string
	Description   string
}

// NewService constructs a new catalog service using the default cache
// location under the user's cache directory.
func NewService() (*Service, error) {
	cachePath, err := defaultCachePath()
	if err != nil {
		return nil, err
	}

	c, err := loadCache(cachePath)
	if err != nil {
		return nil, err
	}

	return &Service{
		client: &http.Client{Timeout: httpClientTimeout},
		cache:  c,
		clock:  time.Now,
	}, nil
}

// SuggestArtifacts returns artifact suggestions that match the provided prefix.
// Cached results are returned immediately, while stale or missing entries trigger
// a background refresh.
func (s *Service) SuggestArtifacts(prefix string) ([]ArtifactSuggestion, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, nil
	}

	suggestions := s.cache.matchPrefix(prefix)
	if len(suggestions) >= maxSuggestions {
		return suggestions[:maxSuggestions], nil
	}

	if err := s.refreshSuggestions(prefix); err != nil && len(suggestions) == 0 {
		return nil, err
	}

	suggestions = s.cache.matchPrefix(prefix)
	if len(suggestions) > maxSuggestions {
		suggestions = suggestions[:maxSuggestions]
	}
	return suggestions, nil
}

// ResolveArtifact fetches the latest metadata for a coordinate, optionally using
// cache data when it is fresh enough.
func (s *Service) ResolveArtifact(groupID, artifactID string) (*ArtifactMetadata, error) {
	key := cacheKey(groupID, artifactID)
	if entry, ok := s.cache.lookup(key); ok && !entry.stale(s.clock(), cacheTTL) {
		return entry.toMetadata(), nil
	}

	meta, err := s.fetchArtifact(groupID, artifactID)
	if err != nil {
		return nil, err
	}

	s.cache.upsert(meta, s.clock())
	if err := s.cache.persist(); err != nil {
		return nil, err
	}

	return meta, nil
}

func (s *Service) refreshSuggestions(prefix string) error {
	records, err := s.fetchSuggestions(prefix)
	if err != nil {
		return err
	}

	s.cache.bulkUpsert(records, s.clock())
	return s.cache.persist()
}

func (s *Service) fetchSuggestions(prefix string) ([]*ArtifactMetadata, error) {
	query := fmt.Sprintf("g:%s* OR a:%s*", escapeQuery(prefix), escapeQuery(prefix))
	params := baseQueryParams()
	params.Set("q", query)

	endpoint := fmt.Sprintf("%s?%s", mavenSearchURL, params.Encode())

	var payload mavenSearchResponse
	if err := s.performRequest(endpoint, &payload); err != nil {
		return nil, err
	}

	records := make([]*ArtifactMetadata, 0, len(payload.Response.Docs))
	for _, doc := range payload.Response.Docs {
		records = append(records, &ArtifactMetadata{
			GroupID:       doc.GroupID,
			ArtifactID:    doc.ArtifactID,
			LatestVersion: doc.LatestVersion,
			Description:   doc.description(),
		})
	}

	return records, nil
}

func (s *Service) fetchArtifact(groupID, artifactID string) (*ArtifactMetadata, error) {
	params := baseQueryParams()
	params.Set("q", fmt.Sprintf("g:\"%s\" AND a:\"%s\"", escapeQuery(groupID), escapeQuery(artifactID)))
	params.Set("core", "gav")

	endpoint := fmt.Sprintf("%s?%s", mavenSearchURL, params.Encode())

	var payload mavenGAVResponse
	if err := s.performRequest(endpoint, &payload); err != nil {
		return nil, err
	}

	if len(payload.Response.Docs) == 0 {
		return nil, fmt.Errorf("artifact not found: %s:%s", groupID, artifactID)
	}

	versions := make([]string, 0, len(payload.Response.Docs))
	for _, doc := range payload.Response.Docs {
		versions = append(versions, doc.Version)
	}
	sort.SliceStable(versions, func(i, j int) bool {
		return versions[i] > versions[j]
	})

	return &ArtifactMetadata{
		GroupID:       groupID,
		ArtifactID:    artifactID,
		LatestVersion: versions[0],
		Versions:      versions,
		Description:   payload.Response.Docs[0].description(),
	}, nil
}

func (s *Service) performRequest(endpoint string, out interface{}) error {
	resp, err := s.client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("maven search returned %s", resp.Status)
	}

	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(out)
}

func defaultCachePath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFileName), nil
}

func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "jpm"), nil
}

// escapeQuery sanitizes a Solr query component.
func escapeQuery(input string) string {
	replacer := strings.NewReplacer(
		":", "\\:",
		" ", "\\ ",
		"\"", "\\\"",
	)
	return replacer.Replace(input)
}

func baseQueryParams() url.Values {
	params := url.Values{}
	params.Set("rows", fmt.Sprintf("%d", mavenDefaultRows))
	params.Set("wt", "json")
	return params
}

type mavenSearchResponse struct {
	Response struct {
		Docs []searchDoc `json:"docs"`
	} `json:"response"`
}

type mavenGAVResponse struct {
	Response struct {
		Docs []gavDoc `json:"docs"`
	} `json:"response"`
}

type searchDoc struct {
	ID            string   `json:"id"`
	GroupID       string   `json:"g"`
	ArtifactID    string   `json:"a"`
	LatestVersion string   `json:"latestVersion"`
	Text          []string `json:"text"`
}

func (d searchDoc) description() string {
	if len(d.Text) == 0 {
		return ""
	}
	return d.Text[0]
}

type gavDoc struct {
	GroupID    string   `json:"g"`
	ArtifactID string   `json:"a"`
	Version    string   `json:"v"`
	Text       []string `json:"text"`
}

func (d gavDoc) description() string {
	if len(d.Text) == 0 {
		return ""
	}
	return d.Text[0]
}

type cache struct {
	path string
	dat  *cacheData
	mu   sync.RWMutex
}

type cacheData struct {
	Version int                    `json:"version"`
	Entries map[string]*cacheEntry `json:"entries"`
}

type cacheEntry struct {
	GroupID       string    `json:"groupId"`
	ArtifactID    string    `json:"artifactId"`
	LatestVersion string    `json:"latestVersion"`
	Versions      []string  `json:"versions"`
	Description   string    `json:"description"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (e *cacheEntry) stale(now time.Time, ttl time.Duration) bool {
	return now.Sub(e.UpdatedAt) > ttl
}

func (e *cacheEntry) toMetadata() *ArtifactMetadata {
	return &ArtifactMetadata{
		GroupID:       e.GroupID,
		ArtifactID:    e.ArtifactID,
		LatestVersion: e.LatestVersion,
		Versions:      append([]string(nil), e.Versions...),
		Description:   e.Description,
	}
}

func loadCache(path string) (*cache, error) {
	data := &cacheData{
		Version: cacheVersion,
		Entries: make(map[string]*cacheEntry),
	}

	if bytes, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(bytes, data); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return &cache{path: path, dat: data}, nil
}

func (c *cache) matchPrefix(prefix string) []ArtifactSuggestion {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lower := strings.ToLower(prefix)
	suggestions := make([]ArtifactSuggestion, 0)
	for _, entry := range c.dat.Entries {
		coord := cacheKey(entry.GroupID, entry.ArtifactID)
		if strings.Contains(strings.ToLower(coord), lower) || strings.Contains(strings.ToLower(entry.Description), lower) {
			suggestions = append(suggestions, ArtifactSuggestion{
				GroupID:       entry.GroupID,
				ArtifactID:    entry.ArtifactID,
				LatestVersion: entry.LatestVersion,
				Description:   entry.Description,
			})
		}
	}
	return suggestions
}

func (c *cache) lookup(key string) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.dat.Entries[key]
	return entry, ok
}

func (c *cache) upsert(meta *ArtifactMetadata, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dat.Entries[cacheKey(meta.GroupID, meta.ArtifactID)] = &cacheEntry{
		GroupID:       meta.GroupID,
		ArtifactID:    meta.ArtifactID,
		LatestVersion: meta.LatestVersion,
		Versions:      append([]string(nil), meta.Versions...),
		Description:   meta.Description,
		UpdatedAt:     now,
	}
}

func (c *cache) bulkUpsert(records []*ArtifactMetadata, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, meta := range records {
		entry, ok := c.dat.Entries[cacheKey(meta.GroupID, meta.ArtifactID)]
		if !ok {
			entry = &cacheEntry{}
			c.dat.Entries[cacheKey(meta.GroupID, meta.ArtifactID)] = entry
		}
		entry.GroupID = meta.GroupID
		entry.ArtifactID = meta.ArtifactID
		entry.LatestVersion = meta.LatestVersion
		entry.Description = meta.Description
		if len(meta.Versions) > 0 {
			entry.Versions = append([]string(nil), meta.Versions...)
		}
		entry.UpdatedAt = now
	}
}

func (c *cache) persist() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(c.dat, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.path, bytes, 0o644)
}

func cacheKey(groupID, artifactID string) string {
	return fmt.Sprintf("%s:%s", groupID, artifactID)
}
