package main

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type StoreProducerFunc func() Storer

type MessageEntry struct {
	Data      []byte
	Timestamp time.Time
	Offset    int64
}

type Storer interface {
	Push([]byte) (int64, error)
	Get(int64) (*MessageEntry, error)
	GetRange(fromOffset, toOffset int64) ([]*MessageEntry, error)
	Len() int64
	Size() int64
	Cleanup(before time.Time) error
}

type MemoryStore struct {
	mu          sync.RWMutex
	data        []*MessageEntry
	size        int64
	maxSize     int64
	config      *Config
	lastCleanup time.Time
}

func (s *MemoryStore) enforceRetention() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if now.Sub(s.lastCleanup) < s.config.CheckInterval {
		return
	}

	cutoff := now.Add(-s.config.RetentionPeriod)
	var validMessages []*MessageEntry
	var newSize int64

	for _, msg := range s.data {
		if msg.Timestamp.After(cutoff) {
			validMessages = append(validMessages, msg)
			newSize += int64(len(msg.Data))
		}
	}

	s.data = validMessages
	s.size = newSize
	s.lastCleanup = now

	slog.Info("retention cleanup completed",
		"messages_kept", len(validMessages),
		"current_size", newSize)
}

func NewMemoryStore(cfg *Config) *MemoryStore {
	store := &MemoryStore{
		data:    make([]*MessageEntry, 0),
		maxSize: cfg.MaxPartitionSize,
		config:  cfg,
	}
	go store.retentionLoop()
	return store
}

func (s *MemoryStore) retentionLoop() {
	ticker := time.NewTicker(s.config.CheckInterval)
	for range ticker.C {
		s.enforceRetention()
	}
}

func (s *MemoryStore) Push(b []byte) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.size+int64(len(b)) > s.maxSize {
		return 0, fmt.Errorf("store size limit exceeded")
	}

	entry := &MessageEntry{
		Data:      b,
		Timestamp: time.Now(),
		Offset:    int64(len(s.data)),
	}

	s.data = append(s.data, entry)
	s.size += int64(len(b))

	return entry.Offset, nil
}

func (s *MemoryStore) Get(offset int64) (*MessageEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if offset < 0 || offset >= int64(len(s.data)) {
		return nil, fmt.Errorf("invalid offset")
	}
	return s.data[offset], nil
}

func (s *MemoryStore) GetRange(fromOffset, toOffset int64) ([]*MessageEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if fromOffset < 0 || toOffset >= int64(len(s.data)) || fromOffset > toOffset {
		return nil, fmt.Errorf("invalid offset range")
	}

	return s.data[fromOffset : toOffset+1], nil
}

func (s *MemoryStore) Len() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.data))
}

func (s *MemoryStore) Size() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.size
}

func (s *MemoryStore) Cleanup(before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var validMessages []*MessageEntry
	var newSize int64

	for _, msg := range s.data {
		if msg.Timestamp.After(before) {
			validMessages = append(validMessages, msg)
			newSize += int64(len(msg.Data))
		}
	}

	s.data = validMessages
	s.size = newSize
	return nil
}
