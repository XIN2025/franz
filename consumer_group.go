package main

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

type ConsumerGroup struct {
	id              string
	members         map[string]Peer
	mutex           sync.RWMutex
	offsets         map[string]map[int]int64
	activeCount     atomic.Int32
	partitionOwners map[string]map[int]string
	rebalanceMu     sync.RWMutex
	server          *Server
	topics          map[string]struct{} // Track subscribed topics
}

func NewConsumerGroup(id string, server *Server) *ConsumerGroup {
	return &ConsumerGroup{
		id:              id,
		members:         make(map[string]Peer),
		offsets:         make(map[string]map[int]int64),
		partitionOwners: make(map[string]map[int]string),
		server:          server,
		topics:          make(map[string]struct{}),
	}
}

func (g *ConsumerGroup) RemoveMember(peerID string) {
	g.mutex.Lock()
	if _, exists := g.members[peerID]; exists {
		// Clear partition assignments for the leaving member
		g.rebalanceMu.Lock()
		for topic, partitions := range g.partitionOwners {
			for partition, owner := range partitions {
				if owner == peerID {
					delete(g.partitionOwners[topic], partition)
				}
			}
		}
		g.rebalanceMu.Unlock()

		delete(g.members, peerID)
		g.activeCount.Add(-1)
	}
	g.mutex.Unlock()

	// Trigger rebalance for all topics
	for topic := range g.offsets {
		partitionCount := len(g.offsets[topic])
		if partitionCount > 0 {
			g.rebalancePartitions(topic, partitionCount)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Add helper method to get partition store
func (g *ConsumerGroup) getPartitionStore(topic string, partition int) (Storer, error) {
	if t, exists := g.server.topics[topic]; exists {
		if partition < len(t.partitions) {
			return t.partitions[partition].store, nil
		}
	}
	return nil, fmt.Errorf("partition store not found")
}

func (g *ConsumerGroup) getPartitionOwner(topic string, partition int) string {
	g.rebalanceMu.RLock()
	defer g.rebalanceMu.RUnlock()

	if topicPartitions, exists := g.partitionOwners[topic]; exists {
		if owner, exists := topicPartitions[partition]; exists {
			return owner
		}
	}
	return ""
}

func (g *ConsumerGroup) rebalancePartitions(topic string, numPartitions int) {
	g.rebalanceMu.Lock()
	defer g.rebalanceMu.Unlock()

	g.mutex.RLock()
	members := make([]string, 0, len(g.members))
	for memberID := range g.members {
		members = append(members, memberID)
	}
	g.mutex.RUnlock()

	if len(members) == 0 {
		slog.Warn("No members available for rebalancing", "topic", topic)
		return
	}

	partitionsPerConsumer := numPartitions / len(members)
	extraPartitions := numPartitions % len(members)
	currentPartition := 0

	g.partitionOwners[topic] = make(map[int]string) // Clear existing assignments
	for i, memberID := range members {
		numPartitionsForMember := partitionsPerConsumer
		if i < extraPartitions {
			numPartitionsForMember++
		}

		for j := 0; j < numPartitionsForMember && currentPartition < numPartitions; j++ {
			g.partitionOwners[topic][currentPartition] = memberID
			slog.Info("Partition assigned", "topic", topic, "partition", currentPartition, "owner", memberID)
			currentPartition++
		}
	}
}

func (g *ConsumerGroup) notifyConsumersOfAssignments(topic string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	g.rebalanceMu.RLock()
	assignments := g.partitionOwners[topic]
	g.rebalanceMu.RUnlock()

	for memberID, peer := range g.members {
		wsPeer, ok := peer.(*WSPeer)
		if !ok {
			continue
		}

		ownedPartitions := make([]int, 0)
		for partition, owner := range assignments {
			if owner == memberID {
				ownedPartitions = append(ownedPartitions, partition)
			}
		}

		wsPeer.assignedPartitionsMu.Lock()
		wsPeer.assignedPartitions[topic] = ownedPartitions
		wsPeer.assignedPartitionsMu.Unlock()

		wsMsg := WSMessage{
			Action:        "partition_assignment",
			Topics:        []string{topic},
			ConsumerGroup: g.id,
			Partitions:    ownedPartitions,
			Success:       true,
		}
		if err := peer.Send(wsMsg); err != nil {
			slog.Error("Failed to send partition assignment", "peer_id", memberID, "err", err)
		} else {
			slog.Info("Sent partition assignment",
				"peer_id", memberID,
				"topic", topic,
				"partitions", ownedPartitions)
		}
	}
}

func (g *ConsumerGroup) AddMember(peer Peer) {
	g.mutex.Lock()
	g.members[peer.ID()] = peer
	g.activeCount.Add(1)
	g.mutex.Unlock()

	slog.Info("Added member to consumer group", "peer_id", peer.ID(), "group", g.id)

	wsPeer, ok := peer.(*WSPeer)
	if !ok {
		slog.Error("Peer is not a WebSocket peer", "peer_id", peer.ID())
		return
	}

	// Subscribe to topics
	wsPeer.topicsMu.RLock()
	topics := make([]string, 0, len(wsPeer.topics))
	for topic := range wsPeer.topics {
		topics = append(topics, topic)
	}
	wsPeer.topicsMu.RUnlock()

	// Add topics to group tracking
	g.mutex.Lock()
	for _, topic := range topics {
		g.topics[topic] = struct{}{}

		// Initialize offsets map if not exists
		if _, exists := g.offsets[topic]; !exists {
			g.offsets[topic] = make(map[int]int64)
		}

		// Initialize partition owners if not exists
		if _, exists := g.partitionOwners[topic]; !exists {
			g.partitionOwners[topic] = make(map[int]string)
		}
	}
	g.mutex.Unlock()

	// Trigger rebalance and assignments
	for _, topic := range topics {
		// Get partition count for topic
		g.server.topicsMu.RLock()
		t, exists := g.server.topics[topic]
		partitionCount := 0
		if exists {
			partitionCount = len(t.partitions)
		}
		g.server.topicsMu.RUnlock()

		if partitionCount > 0 {
			g.rebalancePartitions(topic, partitionCount)
			g.notifyConsumersOfAssignments(topic)
		}
	}

	// Send subscription confirmation
	wsMsg := WSMessage{
		Action:        "subscribed",
		ConsumerGroup: g.id,
		Topics:        topics,
		Success:       true,
	}
	if err := peer.Send(wsMsg); err != nil {
		slog.Error("Failed to send subscription confirmation", "peer_id", peer.ID(), "err", err)
	}

	// Send historical messages for assigned partitions
	for _, topic := range topics {
		g.sendHistoricalMessagesToMember(wsPeer, topic)
	}
}
func (g *ConsumerGroup) sendHistoricalMessagesForTopic(peer *WSPeer, topic string) {
	g.rebalanceMu.RLock()
	owners := g.partitionOwners[topic]
	g.rebalanceMu.RUnlock()

	assignedPartitions := make([]int, 0)
	for partition, owner := range owners {
		if owner == peer.ID() {
			assignedPartitions = append(assignedPartitions, partition)
		}
	}

	for _, partition := range assignedPartitions {
		g.sendHistoricalMessages(peer, topic, partition)
	}
}

func (g *ConsumerGroup) isPartitionOwner(topic string, partition int, peerID string) bool {
	g.rebalanceMu.RLock()
	defer g.rebalanceMu.RUnlock()

	owners, exists := g.partitionOwners[topic]
	if !exists {
		return false
	}

	owner, ok := owners[partition]
	return ok && owner == peerID
}

func (g *ConsumerGroup) getActiveMembers() []string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	members := make([]string, 0, len(g.members))
	for memberID := range g.members {
		members = append(members, memberID)
	}
	return members
}

func (g *ConsumerGroup) rebalancePartitionsForAllTopics() {
	g.mutex.RLock()
	topics := make([]string, 0, len(g.offsets))
	for topic := range g.offsets {
		topics = append(topics, topic)
	}
	g.mutex.RUnlock()

	for _, topic := range topics {
		partitionCount := len(g.server.topics[topic].partitions)
		g.rebalancePartitions(topic, partitionCount)
	}
}

func (g *ConsumerGroup) handleSubscribe(peer *WSPeer, topics []string) {
	slog.Info("Handling subscription", "peer_id", peer.ID(), "topics", topics)

	// Initialize group's topic tracking
	g.mutex.Lock()
	for _, topic := range topics {
		g.topics[topic] = struct{}{}

		// Initialize offsets if not exists
		if _, exists := g.offsets[topic]; !exists {
			g.offsets[topic] = make(map[int]int64)
		}
	}
	g.mutex.Unlock()

	// Update peer's topics
	peer.topicsMu.Lock()
	for _, topic := range topics {
		peer.topics[topic] = struct{}{}
	}
	peer.topicsMu.Unlock()

	// Handle each topic
	for _, topic := range topics {
		// Ensure topic exists and get partition count
		g.server.topicsMu.Lock()
		t, exists := g.server.topics[topic]
		if !exists {
			t = NewTopic(topic, g.server.Config)
			g.server.topics[topic] = t
		}
		partitionCount := len(t.partitions)
		g.server.topicsMu.Unlock()

		// Initialize partition owners if not exists
		g.rebalanceMu.Lock()
		if _, exists := g.partitionOwners[topic]; !exists {
			g.partitionOwners[topic] = make(map[int]string)
		}
		g.rebalanceMu.Unlock()

		// Trigger rebalance
		g.rebalancePartitions(topic, partitionCount)

		// Send partition assignments to all members
		g.notifyConsumersOfAssignments(topic)

		// Send historical messages for this topic to the new member
		g.sendHistoricalMessagesToMember(peer, topic)
	}

	// Send subscription confirmation
	wsMsg := WSMessage{
		Action:        "subscribed",
		ConsumerGroup: g.id,
		Topics:        topics,
		Success:       true,
	}
	if err := peer.Send(wsMsg); err != nil {
		slog.Error("Failed to send subscription confirmation", "peer_id", peer.ID(), "err", err)
	}
}

func (g *ConsumerGroup) distributeMessage(topic string, partition int, msg WSMessage) {
	g.rebalanceMu.RLock()
	ownerID, exists := g.partitionOwners[topic][partition]
	g.rebalanceMu.RUnlock()

	if !exists {
		slog.Warn("No owner for partition", "topic", topic, "partition", partition)
		return
	}

	g.mutex.RLock()
	owner, exists := g.members[ownerID]
	g.mutex.RUnlock()

	if exists {
		slog.Info("Distributing message",
			"topic", topic,
			"partition", partition,
			"owner", ownerID,
			"data", msg.Data)

		if err := owner.Send(msg); err != nil {
			slog.Error("Failed to send message to partition owner",
				"owner_id", ownerID,
				"err", err)
		}
	}
}

func (g *ConsumerGroup) sendHistoricalMessagesToMember(peer *WSPeer, topic string) {
	g.rebalanceMu.RLock()
	ownedPartitions := make([]int, 0)
	for partition, owner := range g.partitionOwners[topic] {
		if owner == peer.ID() {
			ownedPartitions = append(ownedPartitions, partition)
		}
	}
	g.rebalanceMu.RUnlock()

	for _, partition := range ownedPartitions {
		g.sendHistoricalMessages(peer, topic, partition)
	}
}

func (g *ConsumerGroup) sendHistoricalMessages(peer *WSPeer, topic string, partition int) {
	store, err := g.getPartitionStore(topic, partition)
	if err != nil {
		slog.Error("Failed to get partition store",
			"topic", topic,
			"partition", partition,
			"err", err)
		return
	}

	size := store.Len()
	if size == 0 {
		return
	}

	messages, err := store.GetRange(0, size-1)
	if err != nil {
		slog.Error("Failed to get historical messages",
			"topic", topic,
			"partition", partition,
			"err", err)
		return
	}

	slog.Info("Sending historical messages",
		"peer_id", peer.ID(),
		"topic", topic,
		"partition", partition,
		"message_count", len(messages))

	for _, msg := range messages {
		wsMsg := WSMessage{
			Action:    "message",
			Topics:    []string{topic},
			Data:      string(msg.Data),
			Success:   true,
			Partition: partition,
			Offset:    msg.Offset,
		}
		if err := peer.Send(wsMsg); err != nil {
			slog.Error("Failed to send historical message",
				"peer_id", peer.ID(),
				"err", err)
		}
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
