package config

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"polaris/internal/ui"
)

// maxResponseSize limits remote YAML include downloads to 10 MB.
const maxResponseSize = 10 << 20

// loadClientCert is set by the platform-specific certstore implementation.
// It loads a client TLS certificate from the OS certificate store.
// Returns the certificate, a cleanup function, and an error.
var loadClientCert func(subject, storeName string) (*tls.Certificate, func(), error)

// cacheEntry stores metadata for a single cached URL.
type cacheEntry struct {
	URL  string `json:"url"`
	ETag string `json:"etag,omitempty"`
	File string `json:"file"`
}

// cacheMetadata is the persistent cache index stored as metadata.json.
type cacheMetadata struct {
	Entries map[string]cacheEntry `json:"entries"`
}

// URLCache manages a local disk cache for remote YAML includes.
// It uses HTTP ETags to avoid unnecessary downloads and supports
// offline operation by falling back to previously cached content.
type URLCache struct {
	dir    string
	meta   cacheMetadata
	used   map[string]bool // URLs accessed during this load cycle
	client *http.Client
}

// newURLCache creates (or opens) the URL cache directory and loads
// existing metadata. The cache lives in %LOCALAPPDATA%\Polaris\cache.
func newURLCache() (*URLCache, error) {
	dir, err := cacheDir()
	if err != nil {
		return nil, err
	}

	c := &URLCache{
		dir:    dir,
		used:   make(map[string]bool),
		client: &http.Client{Timeout: 30 * time.Second},
		meta: cacheMetadata{
			Entries: make(map[string]cacheEntry),
		},
	}

	c.loadMeta()
	return c, nil
}

// cacheDir returns the cache directory path, creating it if needed.
func cacheDir() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		localAppData = filepath.Dir(exe)
	}
	dir := filepath.Join(localAppData, "Polaris", "cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}
	return dir, nil
}

func (c *URLCache) metaPath() string {
	return filepath.Join(c.dir, "metadata.json")
}

func (c *URLCache) loadMeta() {
	data, err := os.ReadFile(c.metaPath())
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &c.meta)
	if c.meta.Entries == nil {
		c.meta.Entries = make(map[string]cacheEntry)
	}
}

func (c *URLCache) saveMeta() {
	data, _ := json.MarshalIndent(c.meta, "", "  ")
	_ = os.WriteFile(c.metaPath(), data, 0600)
}

// urlHash returns a stable, filesystem-safe hash for a URL.
func urlHash(url string) string {
	h := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", h[:16])
}

// Fetch retrieves a URL using the cache. It sends an If-None-Match
// header when a cached version exists. On network errors the cached
// version is returned (offline mode). auth is optional and enables
// basic auth or mTLS for the request. The caller receives the local
// file path to the (possibly cached) YAML content.
func (c *URLCache) Fetch(url string, auth *AuthConfig) (string, error) {
	c.used[url] = true

	hash := urlHash(url)
	cachedFile := filepath.Join(c.dir, hash+".yaml")
	entry, hasCached := c.meta.Entries[url]

	// Build an HTTP client appropriate for the auth method.
	httpClient, cleanup, err := c.buildClient(auth)
	if err != nil {
		return "", fmt.Errorf("building HTTP client for %q: %w", url, err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request for %q: %w", url, err)
	}

	if hasCached && entry.ETag != "" {
		req.Header.Set("If-None-Match", entry.ETag)
	}

	// Apply basic auth header if configured.
	if auth != nil && auth.Type == "basic" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		// Network error → try offline fallback.
		if hasCached {
			if _, statErr := os.Stat(cachedFile); statErr == nil {
				ui.Statusf("OK", "%s \u2013 using cached version (offline)", truncateURL(url))
				return cachedFile, nil
			}
		}
		return "", fmt.Errorf("fetching %q (no cached version available): %w", url, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		ui.Statusf("OK", "%s – not modified (ETag match)", truncateURL(url))
		return cachedFile, nil

	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		if err != nil {
			return "", fmt.Errorf("reading response from %q: %w", url, err)
		}
		if err := os.WriteFile(cachedFile, body, 0600); err != nil {
			return "", fmt.Errorf("caching %q: %w", url, err)
		}

		c.meta.Entries[url] = cacheEntry{
			URL:  url,
			ETag: resp.Header.Get("ETag"),
			File: hash + ".yaml",
		}
		c.saveMeta()

		ui.Statusf("OK", "%s – downloaded", truncateURL(url))
		return cachedFile, nil

	default:
		if hasCached {
			if _, statErr := os.Stat(cachedFile); statErr == nil {
				ui.Statusf("OK", "%s – HTTP %d, using cached version", truncateURL(url), resp.StatusCode)
				return cachedFile, nil
			}
		}
		return "", fmt.Errorf("fetching %q: HTTP %d", url, resp.StatusCode)
	}
}

// Cleanup removes cached entries whose URLs were not accessed during
// this load cycle. This keeps the cache directory free of stale files.
func (c *URLCache) Cleanup() {
	for url, entry := range c.meta.Entries {
		if !c.used[url] {
			_ = os.Remove(filepath.Join(c.dir, entry.File))
			delete(c.meta.Entries, url)
		}
	}
	c.saveMeta()
}

// buildClient returns an HTTP client configured for the given auth.
// For mTLS, it also returns a cleanup function to free the cert handle.
func (c *URLCache) buildClient(auth *AuthConfig) (*http.Client, func(), error) {
	if auth == nil || auth.Type == "" || auth.Type == "basic" {
		return c.client, nil, nil
	}
	client, cleanup, err := BuildAuthClient(auth)
	if err != nil {
		return nil, nil, err
	}
	return client, cleanup, nil
}

// BuildAuthClient creates an HTTP client configured for the given auth.
// For basic auth, a plain client is returned (the caller must set the
// Authorization header on each request). For mTLS, a client with the
// appropriate TLS config is returned along with a cleanup function.
func BuildAuthClient(auth *AuthConfig) (*http.Client, func(), error) {
	if auth == nil || auth.Type == "" || auth.Type == "basic" {
		return &http.Client{Timeout: 30 * time.Second}, nil, nil
	}

	if auth.Type == "mtls" {
		if loadClientCert == nil {
			return nil, nil, errors.New("mTLS is not supported on this platform")
		}
		cert, cleanup, err := loadClientCert(auth.Subject, auth.Store)
		if err != nil {
			return nil, nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{*cert},
		}
		transport := &http.Transport{TLSClientConfig: tlsCfg}
		client := &http.Client{Timeout: 30 * time.Second, Transport: transport}
		return client, cleanup, nil
	}

	return nil, nil, fmt.Errorf("unsupported auth type: %q", auth.Type)
}

// isURL returns true if the string looks like an HTTP(S) URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// truncateURL shortens a URL for display, showing just the filename part.
func truncateURL(url string) string {
	if i := strings.LastIndex(url, "/"); i >= 0 && i < len(url)-1 {
		return url[i+1:]
	}
	return url
}
