package metrics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockRows implements the Rows interface for testing.
type mockRows struct {
	data    [][]any
	index   int
	scanErr error
}

func (m *mockRows) Next() bool {
	m.index++
	return m.index <= len(m.data)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}

	if m.index < 1 || m.index > len(m.data) {
		return errors.New("no rows")
	}

	row := m.data[m.index-1]
	for i, d := range dest {
		if i < len(row) {
			switch v := d.(type) {
			case *string:
				*v = row[i].(string)
			case *int:
				*v = row[i].(int)
			}
		}
	}

	return nil
}

func (m *mockRows) Close() {}

func (m *mockRows) Err() error {
	return nil
}

// mockDBPool implements DBPool for testing.
type mockDBPool struct {
	stats     PoolStats
	queryResp *mockRows
	queryErr  error
}

func (m *mockDBPool) Stats() PoolStats {
	return m.stats
}

func (m *mockDBPool) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	return m.queryResp, nil
}

func TestNewCollector(t *testing.T) {
	t.Run("with default interval", func(t *testing.T) {
		c := NewCollector(nil, nil, "", 0)
		if c.interval != 30*time.Second {
			t.Errorf("expected default interval of 30s, got %v", c.interval)
		}
	})

	t.Run("with custom interval", func(t *testing.T) {
		c := NewCollector(nil, nil, "", 10*time.Second)
		if c.interval != 10*time.Second {
			t.Errorf("expected interval of 10s, got %v", c.interval)
		}
	})

	t.Run("with all components", func(t *testing.T) {
		db := &mockDBPool{}
		c := NewCollector(db, nil, "/tmp/index", 5*time.Second)

		if c.db == nil {
			t.Error("expected db to be set")
		}

		if c.indexPath != "/tmp/index" {
			t.Errorf("expected indexPath to be /tmp/index, got %s", c.indexPath)
		}
	})
}

func TestCollector_StartStop(t *testing.T) {
	db := &mockDBPool{
		stats:     PoolStats{OpenConnections: 10, InUse: 5},
		queryResp: &mockRows{data: [][]any{}},
	}

	c := NewCollector(db, nil, "", 50*time.Millisecond)

	ctx := context.Background()
	c.Start(ctx)

	// Let it run a couple of collection cycles
	time.Sleep(120 * time.Millisecond)

	// Stop should not block
	done := make(chan struct{})

	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Stop() timed out")
	}
}

func TestCollector_ContextCancellation(t *testing.T) {
	db := &mockDBPool{
		stats:     PoolStats{OpenConnections: 10, InUse: 5},
		queryResp: &mockRows{data: [][]any{}},
	}

	c := NewCollector(db, nil, "", 1*time.Hour) // Long interval

	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// Stop should complete quickly
	done := make(chan struct{})

	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Stop() timed out after context cancellation")
	}
}

func TestCollector_CollectDBStats(t *testing.T) {
	db := &mockDBPool{
		stats: PoolStats{
			MaxOpenConnections: 25,
			OpenConnections:    15,
			InUse:              8,
			Idle:               7,
		},
		queryResp: &mockRows{data: [][]any{}},
	}

	c := NewCollector(db, nil, "", time.Hour)
	c.collect(context.Background())

	// Metrics should be recorded without panic
}

func TestCollector_CollectRepoCounts(t *testing.T) {
	db := &mockDBPool{
		stats: PoolStats{},
		queryResp: &mockRows{
			data: [][]any{
				{"indexed", 100},
				{"pending", 5},
				{"failed", 2},
			},
		},
	}

	c := NewCollector(db, nil, "", time.Hour)
	c.collectRepoCounts(context.Background())

	// Should complete without error
}

func TestCollector_CollectConnectionCounts(t *testing.T) {
	db := &mockDBPool{
		stats: PoolStats{},
		queryResp: &mockRows{
			data: [][]any{
				{"github", 3},
				{"gitlab", 2},
			},
		},
	}

	c := NewCollector(db, nil, "", time.Hour)
	c.collectConnectionCounts(context.Background())

	// Should complete without error
}

func TestCollector_CollectJobQueueCounts(t *testing.T) {
	db := &mockDBPool{
		stats: PoolStats{},
		queryResp: &mockRows{
			data: [][]any{
				{"index", 10},
				{"sync", 5},
			},
		},
	}

	c := NewCollector(db, nil, "", time.Hour)
	c.collectJobQueueCounts(context.Background())

	// Should complete without error
}

func TestCollector_CollectZoektStats(t *testing.T) {
	// Create a temp directory with some test files
	tmpDir, err := os.MkdirTemp("", "zoekt-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some mock shard files
	files := []struct {
		name string
		size int
	}{
		{"repo1_v16.00000.zoekt", 1024},
		{"repo2_v16.00000.zoekt", 2048},
		{"repo3_v16.00000.zoekt", 4096},
		{"not-a-shard.txt", 100},
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)

		data := make([]byte, f.size)

		err := os.WriteFile(path, data, 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}

	c := NewCollector(nil, nil, tmpDir, time.Hour)
	shards, size := c.collectZoektStats()

	if shards != 3 {
		t.Errorf("expected 3 shards, got %d", shards)
	}

	expectedSize := int64(1024 + 2048 + 4096)
	if size != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, size)
	}
}

func TestCollector_CollectZoektStats_EmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zoekt-empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c := NewCollector(nil, nil, tmpDir, time.Hour)
	shards, size := c.collectZoektStats()

	if shards != 0 {
		t.Errorf("expected 0 shards, got %d", shards)
	}

	if size != 0 {
		t.Errorf("expected size 0, got %d", size)
	}
}

func TestCollector_CollectZoektStats_NonexistentDir(t *testing.T) {
	c := NewCollector(nil, nil, "/nonexistent/path", time.Hour)
	shards, size := c.collectZoektStats()

	if shards != 0 {
		t.Errorf("expected 0 shards for nonexistent dir, got %d", shards)
	}

	if size != 0 {
		t.Errorf("expected size 0 for nonexistent dir, got %d", size)
	}
}

func TestCollector_QueryError(t *testing.T) {
	db := &mockDBPool{
		stats:    PoolStats{},
		queryErr: errors.New("database connection error"),
	}

	c := NewCollector(db, nil, "", time.Hour)

	// These should handle the error gracefully without panic
	c.collectRepoCounts(context.Background())
	c.collectConnectionCounts(context.Background())
	c.collectJobQueueCounts(context.Background())
}

func TestDBPoolAdapter(t *testing.T) {
	t.Run("with StatsFunc", func(t *testing.T) {
		adapter := &DBPoolAdapter{
			StatsFunc: func() PoolStats {
				return PoolStats{OpenConnections: 10, InUse: 5}
			},
		}

		stats := adapter.Stats()
		if stats.OpenConnections != 10 {
			t.Errorf("expected OpenConnections=10, got %d", stats.OpenConnections)
		}

		if stats.InUse != 5 {
			t.Errorf("expected InUse=5, got %d", stats.InUse)
		}
	})

	t.Run("without StatsFunc", func(t *testing.T) {
		adapter := &DBPoolAdapter{}
		stats := adapter.Stats()

		// Should return zero values
		if stats.OpenConnections != 0 {
			t.Errorf("expected OpenConnections=0, got %d", stats.OpenConnections)
		}
	})

	t.Run("with QueryFunc", func(t *testing.T) {
		expectedRows := &mockRows{}
		adapter := &DBPoolAdapter{
			QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
				return expectedRows, nil
			},
		}

		rows, err := adapter.Query(context.Background(), "SELECT 1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if rows != expectedRows {
			t.Error("expected same rows instance")
		}
	})
}
