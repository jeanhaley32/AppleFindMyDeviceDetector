package main

import (
	"sync"
	"testing"
)

// TestScannerScanCount_ConcurrentAccess reproduces the write/read pattern from
// scan() (writer) and startScan() (reader): scanCount is written in one
// goroutine and read in another while scanning is in progress. Before the
// atomic.Int64 fix, `go test -race` flagged this as a data race
// (see beads-yly / issue #11). This test fails under -race on a regression.
func TestScannerScanCount_ConcurrentAccess(t *testing.T) {
	s := &scanner{}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.scanCount.Store(0)
		for i := 0; i < 1000; i++ {
			s.scanCount.Add(1)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = s.scanCount.Load()
		}
	}()

	wg.Wait()

	if got := s.scanCount.Load(); got != 1000 {
		t.Fatalf("scanCount = %d, want 1000", got)
	}
}
