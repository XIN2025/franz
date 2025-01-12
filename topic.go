package main

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"
)

type Topic struct {
	name       string
	partitions []*Partition
	mutex      sync.RWMutex
	peers      map[string]Peer
	peersMu    sync.RWMutex
	created    time.Time
	config     *Config
}

func NewTopic(name string, cfg *Config) *Topic {
	partitions := make([]*Partition, cfg.DefaultPartitions)
	for i := 0; i < cfg.DefaultPartitions; i++ {
		partitions[i] = NewPartition(i, cfg.StoreProducerFunc(cfg))
	}

	return &Topic{
		name:       name,
		partitions: partitions,
		peers:      make(map[string]Peer),
		created:    time.Now(),
		config:     cfg,
	}
}

func (t *Topic) getPartition(data []byte) int {
	hasher := fnv.New32a()
	hasher.Write(data)
	return int(hasher.Sum32()) % len(t.partitions)
}

func (t *Topic) Push(data []byte) (int, int64, error) {
	partitionID := t.getPartition(data)

	t.mutex.RLock()
	partition := t.partitions[partitionID]
	t.mutex.RUnlock()

	// Check partition size before pushing
	if partition.store.Size() >= t.config.MaxPartitionSize {
		return 0, 0, fmt.Errorf("partition %d size limit exceeded", partitionID)
	}

	offset, err := partition.Push(data)
	if err != nil {
		return 0, 0, err
	}

	wsMsg := WSMessage{
		Action: "message",
		Topics: []string{t.name},
		Data:   string(data),
		// DataRaw:   data,
		Success:   true,
		Partition: partitionID,
		Offset:    offset,
	}

	t.peersMu.RLock()
	defer t.peersMu.RUnlock()

	// Broadcast to subscribers
	for _, peer := range t.peers {
		if err := peer.Send(wsMsg); err != nil {
			slog.Error("failed to send message to peer",
				"peer_id", peer.ID(),
				"err", err)
		}
	}

	return partitionID, offset, nil
}
