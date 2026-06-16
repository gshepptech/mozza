package cluster

import (
	"log/slog"
	"time"

	"github.com/gshepptech/mozza/internal/store"
)

const (
	snapshotInterval    = 60 * time.Second
	snapshotMaxAge      = 24 * time.Hour
	snapshotCleanupFreq = 10 // cleanup every N saves
)

// SnapshotManager periodically persists the ClusterCache to SQLite
// so that the dashboard can show stale-but-useful data on restart
// before live informers reconnect.
type SnapshotManager struct {
	cache *ClusterCache
	store *store.Store
	stopC chan struct{}
	doneC chan struct{}
}

// NewSnapshotManager creates a SnapshotManager.
func NewSnapshotManager(cache *ClusterCache, st *store.Store) *SnapshotManager {
	return &SnapshotManager{
		cache: cache,
		store: st,
		stopC: make(chan struct{}),
		doneC: make(chan struct{}),
	}
}

// Start begins the background snapshot loop.
func (m *SnapshotManager) Start() {
	go m.run()
}

// Stop signals the background goroutine to exit and waits for it.
func (m *SnapshotManager) Stop() {
	close(m.stopC)
	<-m.doneC
}

// LoadSnapshot reads the latest snapshot from SQLite and populates
// the cache. It is safe to call before Start.
func (m *SnapshotManager) LoadSnapshot() {
	data, ts, err := m.store.LoadLatestClusterSnapshot()
	if err != nil {
		slog.Info("no cluster snapshot to restore", "error", err)
		return
	}
	if err := m.cache.LoadFromSnapshot(data); err != nil {
		slog.Warn("failed to parse cluster snapshot", "error", err)
		return
	}
	slog.Info("restored cluster cache from snapshot",
		"age", time.Since(ts).Round(time.Second))
}

// SaveSnapshot serialises the current cache and writes it to SQLite.
func (m *SnapshotManager) SaveSnapshot() {
	data := m.cache.SerializeSnapshot()
	if data == "{}" {
		return // nothing to persist
	}
	if err := m.store.SaveClusterSnapshot(data); err != nil {
		slog.Warn("failed to save cluster snapshot", "error", err)
	}
}

func (m *SnapshotManager) run() {
	defer close(m.doneC)

	ticker := time.NewTicker(snapshotInterval)
	defer ticker.Stop()

	var saves int
	for {
		select {
		case <-m.stopC:
			m.SaveSnapshot() // final save on shutdown
			return
		case <-ticker.C:
			m.SaveSnapshot()
			saves++
			if saves%snapshotCleanupFreq == 0 {
				if err := m.store.CleanupOldClusterSnapshots(snapshotMaxAge); err != nil {
					slog.Warn("snapshot cleanup failed", "error", err)
				}
			}
		}
	}
}
