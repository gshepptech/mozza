package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gshepptech/mozza/internal/store"
	"github.com/gshepptech/mozza/internal/template"
)

const (
	// defaultIndexURL is the GitHub raw URL for the community recipe index.
	defaultIndexURL = "https://raw.githubusercontent.com/gshepptech/mozza-recipes/main/index.json"
	// indexCacheTTL is how long the cached index is valid before refresh.
	indexCacheTTL = 1 * time.Hour
	// fetchTimeout is the HTTP timeout for fetching the remote index.
	fetchTimeout = 15 * time.Second
	// maxIndexSize is the maximum size of the remote index (1 MB).
	maxIndexSize = 1 << 20
)

// IndexEntry represents a recipe entry from the remote index.
type IndexEntry struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Icon         string                 `json:"icon"`
	Category     string                 `json:"category"`
	Tags         []string               `json:"tags"`
	Source       string                 `json:"source"`
	Variables    []template.TemplateVar `json:"variables"`
	Repo         string                 `json:"repo,omitempty"`
	Official     bool                   `json:"official"`
	MinK8sVer    string                 `json:"min_k8s_ver,omitempty"`
	EstResources string                 `json:"est_resources,omitempty"`
}

// indexCache holds the fetched remote index with cache metadata.
type indexCache struct {
	mu        sync.RWMutex
	entries   []IndexEntry
	fetchedAt time.Time
}

// isStale returns true if the cache needs refreshing.
func (c *indexCache) isStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.fetchedAt) > indexCacheTTL
}

// get returns the cached entries.
func (c *indexCache) get() []IndexEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]IndexEntry, len(c.entries))
	copy(out, c.entries)
	return out
}

// set updates the cache.
func (c *indexCache) set(entries []IndexEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = entries
	c.fetchedAt = time.Now()
}

// fetchIndex downloads the recipe index from the remote URL.
func fetchIndex(ctx context.Context, url string) ([]IndexEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchIndex: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchIndex: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchIndex: HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIndexSize))
	if err != nil {
		return nil, fmt.Errorf("fetchIndex: read body: %w", err)
	}

	var entries []IndexEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("fetchIndex: parse JSON: %w", err)
	}

	return entries, nil
}

// syncToStore persists remote index entries to the store as MarketplaceRecipe
// cache rows, using upsert to avoid duplicates.
func syncToStore(ctx context.Context, st *store.Store, entries []IndexEntry) {
	for _, e := range entries {
		tags := strings.Join(e.Tags, ",")
		content, _ := json.Marshal(e)
		if _, err := st.UpsertMarketplaceRecipe(ctx, e.Name, e.Category, tags, "", string(content)); err != nil {
			slog.Warn("marketplace: failed to cache recipe",
				"name", e.Name, "error", err)
		}
	}
}

// entryToTemplate converts an IndexEntry to a template.Template.
func entryToTemplate(e IndexEntry) template.Template {
	return template.Template{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		Icon:         e.Icon,
		Category:     e.Category,
		Tags:         e.Tags,
		Source:       e.Source,
		Variables:    e.Variables,
		Repo:         e.Repo,
		Official:     e.Official,
		MinK8sVer:    e.MinK8sVer,
		EstResources: e.EstResources,
	}
}
