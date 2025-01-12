package main

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func generateID() string {
	return uuid.New().String()
}

type Peer interface {
	Send(WSMessage) error
	Close() error
	ID() string
	GetMessageCount() int64
}

type WSPeer struct {
	conn                 *websocket.Conn
	server               *Server
	id                   string
	stopCh               chan struct{}
	topics               map[string]struct{}
	topicsMu             sync.RWMutex
	closed               atomic.Bool
	messageCount         atomic.Int64
	consumerGroup        *ConsumerGroup
	assignedPartitions   map[string][]int // topic -> partitions
	assignedPartitionsMu sync.RWMutex
	debugID              string // Short ID for logging
}

func NewWSPeer(conn *websocket.Conn, s *Server) *WSPeer {
	fullID := generateID()
	shortID := fullID[:8] // Use first 8 chars for readable logging
	p := &WSPeer{
		conn:               conn,
		server:             s,
		id:                 fullID,
		debugID:            shortID,
		stopCh:             make(chan struct{}),
		topics:             make(map[string]struct{}),
		assignedPartitions: make(map[string][]int),
	}
	slog.Info("Created new peer", "debug_id", p.debugID, "full_id", p.id)
	go p.readLoop()
	return p
}

func (p *WSPeer) GetMessageCount() int64 {
	return p.messageCount.Load()
}

func (p *WSPeer) readLoop() {
	defer func() {
		p.Close()
		close(p.stopCh)
	}()

	for {
		if p.closed.Load() {
			return
		}

		var msg WSMessage
		if err := p.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Error("ws peer read error", "peer_id", p.id, "err", err)
			}
			return
		}

		if err := p.handleMessage(msg); err != nil {
			slog.Error("ws handle message error", "peer_id", p.id, "err", err)
		}
	}
}

func (p *WSPeer) handleMessage(msg WSMessage) error {
	slog.Info("→ Received WebSocket message",
		"peer_debug_id", p.debugID,
		"action", msg.Action,
		"consumer_group", msg.ConsumerGroup,
		"topics", msg.Topics)

	switch msg.Action {
	case "subscribe":
		// Update peer's topics before joining group
		p.topicsMu.Lock()
		for _, topic := range msg.Topics {
			p.topics[topic] = struct{}{}
		}
		p.topicsMu.Unlock()

		if err := p.joinConsumerGroup(msg.ConsumerGroup); err != nil {
			return p.Send(WSMessage{
				Action:  "error",
				Error:   err.Error(),
				Success: false,
			})
		}
		return nil

	case "unsubscribe":
		if p.consumerGroup != nil {
			p.leaveConsumerGroup()
		}
		p.server.RemovePeerFromTopics(p, msg.Topics...)
		return p.Send(WSMessage{
			Action:  "unsubscribed",
			Topics:  msg.Topics,
			Success: true,
		})

	default:
		return p.Send(WSMessage{
			Action:  "error",
			Error:   fmt.Sprintf("unknown action: %s", msg.Action),
			Success: false,
		})
	}
}

func (p *WSPeer) joinConsumerGroup(groupID string) error {
	if p.consumerGroup != nil {
		return fmt.Errorf("peer already in consumer group")
	}

	p.server.groupsMu.Lock()
	group, exists := p.server.consumerGroups[groupID]
	if !exists {
		group = NewConsumerGroup(groupID, p.server)
		p.server.consumerGroups[groupID] = group
	}
	p.server.groupsMu.Unlock()

	p.consumerGroup = group
	group.AddMember(p)

	return nil
}

func (p *WSPeer) leaveConsumerGroup() {
	if p.consumerGroup == nil {
		return
	}

	p.consumerGroup.RemoveMember(p.id)
	p.consumerGroup = nil
}

func (p *WSPeer) Send(wsMsg WSMessage) error {
	if p.closed.Load() {
		slog.Warn("Attempt to send to closed peer", "peer_id", p.ID())
		return fmt.Errorf("peer is closed")
	}

	slog.Info("Sending WebSocket message", "peer_id", p.ID(), "message_action", wsMsg.Action)
	if err := p.conn.WriteJSON(wsMsg); err != nil {
		slog.Error("WebSocket send failed", "peer_id", p.ID(), "err", err)
		p.Close()
		return err
	}

	p.messageCount.Add(1)
	return nil
}

func (p *WSPeer) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}

	if p.consumerGroup != nil {
		p.leaveConsumerGroup()
	}

	p.topicsMu.Lock()
	for topic := range p.topics {
		delete(p.topics, topic)
	}
	p.topicsMu.Unlock()

	close(p.stopCh)
	return p.conn.Close()
}

func (p *WSPeer) ID() string {
	return p.id
}
