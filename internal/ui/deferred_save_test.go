package ui

import (
	"garden_cosmos/internal/garden"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestDeferredSaveCoalescesAndFlushes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := &deferredSave{path: path, delay: time.Hour}
	defer s.Flush()
	for i := 0; i < 100; i++ {
		s.Schedule(i)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("input path wrote synchronously")
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	var got int
	if err := garden.Load(path, &got); err != nil || got != 99 {
		t.Fatal(got, err)
	}
}
func TestDeferredSaveConcurrentLatestWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := &deferredSave{path: path, delay: time.Millisecond}
	defer s.Flush()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) { defer wg.Done(); s.Schedule(n); _ = s.Flush() }(i)
	}
	wg.Wait()
	s.Schedule(999)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	var got int
	if err := garden.Load(path, &got); err != nil || got != 999 {
		t.Fatal(got, err)
	}
}
