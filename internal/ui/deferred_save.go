package ui

import (
	"garden_cosmos/internal/garden"
	"sync"
	"time"
)

// Coalesce immutable snapshots without doing disk I/O on the input event path.
// Flush serializes with any timer write, so closing cannot leave an older value.
type deferredSave struct {
	mu      sync.Mutex
	writeMu sync.Mutex
	path    string
	value   any
	pending bool
	timer   *time.Timer
	err     error
	delay   time.Duration
}

func (s *deferredSave) Schedule(value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = value
	s.pending = true
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.delay, func() { _ = s.Flush() })
}
func (s *deferredSave) Flush() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if !s.pending {
		err := s.err
		s.mu.Unlock()
		return err
	}
	value := s.value
	s.pending = false
	s.mu.Unlock()
	err := garden.Save(s.path, value)
	s.mu.Lock()
	s.err = err
	if err != nil && !s.pending {
		s.pending = true
	}
	s.mu.Unlock()
	return err
}
func (u *UI) flushCalculator() error {
	if u.calculatorSave != nil {
		return u.calculatorSave.Flush()
	}
	return nil
}
