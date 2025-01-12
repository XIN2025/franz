package main

import (
	"sync"
	"sync/atomic"
)

type ConsumerGroup struct {
	id          string
	members     map[string]Peer
	mutex       sync.RWMutex
	offsets     map[string]map[int]int64 // topic -> partition -> offset
	activeCount atomic.Int32
}

func NewConsumerGroup(id string) *ConsumerGroup {
	return &ConsumerGroup{
		id:      id,
		members: make(map[string]Peer),
		offsets: make(map[string]map[int]int64),
	}
}

func (g *ConsumerGroup) AddMember(peer Peer) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.members[peer.ID()] = peer
	g.activeCount.Add(1)

	// Initialize offsets if needed
	for topic, partOffsets := range g.offsets {
		for partition, offset := range partOffsets {
			wsMsg := WSMessage{
				Action:        "offset_update",
				Topics:        []string{topic},
				ConsumerGroup: g.id,
				Partition:     partition,
				Offset:        offset,
				Success:       true,
			}
			peer.Send(wsMsg)
		}
	}
}

func (g *ConsumerGroup) RemoveMember(peerID string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if _, exists := g.members[peerID]; exists {
		delete(g.members, peerID)
		g.activeCount.Add(-1)
	}
}

func (g *ConsumerGroup) UpdateOffset(topic string, partition int, offset int64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if _, exists := g.offsets[topic]; !exists {
		g.offsets[topic] = make(map[int]int64)
	}
	g.offsets[topic][partition] = offset
}
