package workflow

import (
	"sync"
	"testing"
)

// NOTE: These tests are intentionally DB-free. They validate the intended Phase 0 semantics:
// - at-least-once delivery is safe via durable idempotency
// - per-business serialization prevents racey interleavings inside handlers
//
// Full DB+PubSub integration tests should be added in an environment that can run MySQL + Pub/Sub emulator.

type fakeProcessor struct {
	muByBiz map[string]*sync.Mutex
	mu      sync.Mutex
	seen    map[string]bool
	calls   int
}

func newFakeProcessor() *fakeProcessor {
	return &fakeProcessor{
		muByBiz: map[string]*sync.Mutex{},
		seen:    map[string]bool{},
	}
}

func (p *fakeProcessor) process(businessID, handlerName, messageID string, fn func()) {
	// Serialize per business (models AcquireBusinessPostingLock).
	p.mu.Lock()
	bm := p.muByBiz[businessID]
	if bm == nil {
		bm = &sync.Mutex{}
		p.muByBiz[businessID] = bm
	}
	p.mu.Unlock()

	bm.Lock()
	defer bm.Unlock()

	// Deduplicate (models IdempotencyKey).
	key := businessID + "|" + handlerName + "|" + messageID
	p.mu.Lock()
	if p.seen[key] {
		p.mu.Unlock()
		return
	}
	p.seen[key] = true
	p.mu.Unlock()

	fn()

	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
}

func TestPhase0_DuplicateDelivery_IsProcessedOnce(t *testing.T) {
	p := newFakeProcessor()

	const (
		biz       = "biz-1"
		handler   = "IV"
		messageID = "123"
	)

	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.process(biz, handler, messageID, func() {})
		}()
	}
	wg.Wait()

	if p.calls != 1 {
		t.Fatalf("expected exactly 1 processing call, got %d", p.calls)
	}
}

func TestPhase0_Property_DeterministicUnderConcurrency(t *testing.T) {
	for run := 0; run < 100; run++ {
		p := newFakeProcessor()
		var wg sync.WaitGroup

		// same scenario, repeated concurrently
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				p.process("biz-1", "IV", "1", func() {})
				p.process("biz-1", "BL", "2", func() {})
				p.process("biz-1", "IV", "1", func() {}) // duplicate
			}(i)
		}
		wg.Wait()

		if p.calls != 2 {
			t.Fatalf("run=%d expected 2 unique calls (IV#1, BL#2), got %d", run, p.calls)
		}
	}
}

