package lock

import (
	"os"
	"testing"
	"time"
)

func TestNewDistributedLock_WorkerID(t *testing.T) {
	t.Run("with HOSTNAME env var", func(t *testing.T) {
		originalHostname := os.Getenv("HOSTNAME")
		defer os.Setenv("HOSTNAME", originalHostname)

		os.Setenv("HOSTNAME", "test-worker-1")

		lock := NewDistributedLock(nil, "test-key", 10*time.Second)

		if lock.workerID != "test-worker-1" {
			t.Errorf("workerID = %v, want test-worker-1", lock.workerID)
		}
	})

	t.Run("without HOSTNAME env var generates ID", func(t *testing.T) {
		originalHostname := os.Getenv("HOSTNAME")
		defer os.Setenv("HOSTNAME", originalHostname)

		os.Unsetenv("HOSTNAME")

		lock := NewDistributedLock(nil, "test-key", 10*time.Second)

		if lock.workerID == "" {
			t.Error("workerID should be generated when HOSTNAME is not set")
		}

		if lock.workerID[:7] != "worker-" {
			t.Errorf("workerID should start with 'worker-', got %v", lock.workerID)
		}
	})
}

func TestNewDistributedLock_KeyPrefix(t *testing.T) {
	lock := NewDistributedLock(nil, "my-lock", 10*time.Second)

	expectedKey := "codesearch:lock:my-lock"
	if lock.key != expectedKey {
		t.Errorf("key = %v, want %v", lock.key, expectedKey)
	}
}

func TestNewDistributedLock_TTL(t *testing.T) {
	ttl := 30 * time.Second
	lock := NewDistributedLock(nil, "test", ttl)

	if lock.ttl != ttl {
		t.Errorf("ttl = %v, want %v", lock.ttl, ttl)
	}
}

func TestDistributedLock_Fields(t *testing.T) {
	ttl := 5 * time.Minute
	lock := NewDistributedLock(nil, "test-lock", ttl)

	if lock.client != nil {
		t.Error("client should be nil when passed nil")
	}

	if lock.key != "codesearch:lock:test-lock" {
		t.Errorf("key = %v, want codesearch:lock:test-lock", lock.key)
	}

	if lock.ttl != ttl {
		t.Errorf("ttl = %v, want %v", lock.ttl, ttl)
	}
}
