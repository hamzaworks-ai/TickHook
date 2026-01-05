package executor

import (
	"sync"
	"testing"

	"github.com/cr0hn/tickhook/internal/config"
)

func TestGetDomainSemaphore(t *testing.T) {
	cfg := &config.Config{
		MaxInflight:  100,
		MaxPerDomain: 5,
	}
	exec := &Executor{
		cfg:        cfg,
		domainSems: make(map[string]chan struct{}),
	}

	// Get semaphore for a domain
	sem1 := exec.getDomainSemaphore("example.com")
	if sem1 == nil {
		t.Fatal("Semaphore should not be nil")
	}
	if cap(sem1) != 5 {
		t.Errorf("Semaphore capacity = %d, want 5", cap(sem1))
	}

	// Get same semaphore again
	sem2 := exec.getDomainSemaphore("example.com")
	if sem1 != sem2 {
		t.Error("Should return same semaphore for same domain")
	}

	// Get different semaphore for different domain
	sem3 := exec.getDomainSemaphore("other.com")
	if sem1 == sem3 {
		t.Error("Should return different semaphore for different domain")
	}
}

func TestGetDomainSemaphore_Concurrent(t *testing.T) {
	cfg := &config.Config{
		MaxInflight:  100,
		MaxPerDomain: 5,
	}
	exec := &Executor{
		cfg:        cfg,
		domainSems: make(map[string]chan struct{}),
	}

	var wg sync.WaitGroup
	semaphores := make(chan chan struct{}, 100)

	// Concurrently get semaphores for the same domain
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem := exec.getDomainSemaphore("concurrent.com")
			semaphores <- sem
		}()
	}

	wg.Wait()
	close(semaphores)

	// All semaphores should be the same
	var first chan struct{}
	for sem := range semaphores {
		if first == nil {
			first = sem
		} else if first != sem {
			t.Error("All concurrent calls should return the same semaphore")
		}
	}
}

func TestDomainSemaphore_Limiting(t *testing.T) {
	cfg := &config.Config{
		MaxInflight:  100,
		MaxPerDomain: 2, // Only 2 concurrent per domain
	}
	exec := &Executor{
		cfg:        cfg,
		domainSems: make(map[string]chan struct{}),
	}

	sem := exec.getDomainSemaphore("limited.com")

	// Acquire 2 slots (should succeed)
	sem <- struct{}{}
	sem <- struct{}{}

	// Try to acquire third (should block)
	select {
	case sem <- struct{}{}:
		t.Error("Third acquire should block")
	default:
		// Expected: channel is full
	}

	// Release one
	<-sem

	// Now should be able to acquire
	select {
	case sem <- struct{}{}:
		// Expected: can acquire now
	default:
		t.Error("Should be able to acquire after release")
	}
}
