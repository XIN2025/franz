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
	conn          *websocket.Conn
	server        *Server
	id            string
	stopCh        chan struct{}
	topics        map[string]struct{}
	topicsMu      sync.RWMutex
	closed        atomic.Bool
	messageCount  atomic.Int64
	consumerGroup *ConsumerGroup
}

func NewWSPeer(conn *websocket.Conn, s *Server) *WSPeer {
	p := &WSPeer{
		conn:   conn,
		server: s,
		id:     generateID(),
		stopCh: make(chan struct{}),
		topics: make(map[string]struct{}),
	}
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
	response := WSMessage{
		Success: true,
	}

	switch msg.Action {
	case "subscribe":
		if msg.ConsumerGroup != "" {
			p.joinConsumerGroup(msg.ConsumerGroup, msg.Topics...)
		} else {
			p.server.AddPeerToTopics(p, msg.Topics...)
		}
		response.Action = "subscribed"
		response.Topics = msg.Topics

	case "unsubscribe":
		if p.consumerGroup != nil {
			p.leaveConsumerGroup()
		}
		p.server.RemovePeerFromTopics(p, msg.Topics...)
		response.Action = "unsubscribed"
		response.Topics = msg.Topics

	default:
		response.Success = false
		response.Error = fmt.Sprintf("unknown action: %s", msg.Action)
	}

	return p.conn.WriteJSON(response)
}
func (p *WSPeer) joinConsumerGroup(groupID string, topics ...string) {
	p.server.groupsMu.Lock()
	group, exists := p.server.consumerGroups[groupID]
	if !exists {
		group = NewConsumerGroup(groupID)
		p.server.consumerGroups[groupID] = group
	}
	p.server.groupsMu.Unlock()

	group.AddMember(p)
	p.consumerGroup = group

	// Subscribe to topics
	p.server.AddPeerToTopics(p, topics...)
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
		slog.Error("Attempt to send to closed peer", "peer_id", p.ID())
		return nil
	}

	slog.Info("Sending message to WebSocket", "peer_id", p.ID(), "message_action", wsMsg.Action)
	if err := p.conn.WriteJSON(wsMsg); err != nil {
		slog.Error("Failed to write to WebSocket", "peer_id", p.ID(), "err", err)
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

	return p.conn.Close()
}
func (p *WSPeer) ID() string {
	return p.id
}
