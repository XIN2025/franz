package main

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Config struct {
	HTTPListenAddr    string
	WSListenAddr      string
	StoreProducerFunc func(*Config) Storer
	BufferSize        int
	DefaultPartitions int
	ReplicationFactor int
	RetentionPeriod   time.Duration
	MaxPartitionSize  int64
	CheckInterval     time.Duration
}

func DefaultConfig() *Config {
	cfg := &Config{
		HTTPListenAddr:    ":3000",
		WSListenAddr:      ":4000",
		BufferSize:        1000,
		DefaultPartitions: 4,
		ReplicationFactor: 1,
		RetentionPeriod:   24 * time.Hour,
		MaxPartitionSize:  1024 * 1024 * 1024,
		CheckInterval:     1 * time.Minute,
	}

	cfg.StoreProducerFunc = func(c *Config) Storer {
		return NewMemoryStore(c)
	}

	return cfg
}

type Server struct {
	*Config
	peers          map[string]Peer
	peersMu        sync.RWMutex
	topics         map[string]*Topic
	topicsMu       sync.RWMutex
	consumers      []Consumer
	producers      []Producer
	msgCh          chan Message
	stopCh         chan struct{}
	wg             sync.WaitGroup
	consumerGroups map[string]*ConsumerGroup
	groupsMu       sync.RWMutex
}

func NewServer(cfg *Config) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	s := &Server{
		Config:         cfg,
		peers:          make(map[string]Peer),
		topics:         make(map[string]*Topic),
		consumerGroups: make(map[string]*ConsumerGroup),
		msgCh:          make(chan Message, cfg.BufferSize),
		stopCh:         make(chan struct{}),
	}

	httpProducer := NewHTTPProducer(cfg.HTTPListenAddr, cfg.BufferSize)
	httpProducer.msgCh = s.msgCh
	s.producers = []Producer{httpProducer}
	s.consumers = []Consumer{NewWSConsumer(cfg.WSListenAddr, s)}

	return s, nil
}

func (s *Server) Start() error {
	for _, consumer := range s.consumers {
		if err := consumer.Start(); err != nil {
			return fmt.Errorf("failed to start consumer: %w", err)
		}
	}

	for _, producer := range s.producers {
		if err := producer.Start(); err != nil {
			return fmt.Errorf("failed to start producer: %w", err)
		}
	}

	s.wg.Add(1)
	go s.loop()

	return nil
}

func (s *Server) Stop() error {
	close(s.stopCh)
	s.wg.Wait()

	for _, consumer := range s.consumers {
		if err := consumer.Stop(); err != nil {
			slog.Error("failed to stop consumer", "err", err)
		}
	}

	for _, producer := range s.producers {
		if err := producer.Stop(); err != nil {
			slog.Error("failed to stop producer", "err", err)
		}
	}

	return nil
}

func (s *Server) loop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case msg := <-s.msgCh:
			slog.Info("received message in server loop", "topic", msg.Topic)
			if err := s.handleMessage(msg); err != nil {
				slog.Error("failed to handle message", "err", err)
			}
		}
	}
}

func (s *Server) handleMessage(msg Message) error {
	s.topicsMu.Lock()
	topic, exists := s.topics[msg.Topic]
	if !exists {
		topic = NewTopic(msg.Topic, s.Config)
		s.topics[msg.Topic] = topic
		slog.Info("Created new topic", "topic", msg.Topic)
	}
	s.topicsMu.Unlock()

	partitionID := topic.getPartition(msg.Data)
	partition := topic.partitions[partitionID]

	offset, err := partition.Push(msg.Data)
	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	wsMsg := WSMessage{
		Action:    "message",
		Topics:    []string{msg.Topic},
		Data:      string(msg.Data),
		Success:   true,
		Partition: partitionID,
		Offset:    offset,
	}

	s.groupsMu.RLock()
	for _, group := range s.consumerGroups {
		group.mutex.RLock()
		if _, ok := group.topics[msg.Topic]; ok {
			slog.Info("Distributing message to consumer group",
				"group_id", group.id,
				"topic", msg.Topic,
				"partition", partitionID)
			group.distributeMessage(msg.Topic, partitionID, wsMsg)
		}
		group.mutex.RUnlock()
	}
	s.groupsMu.RUnlock()

	return nil
}

func (s *Server) AddPeerToTopics(p Peer, topics ...string) {
	s.topicsMu.Lock()
	defer s.topicsMu.Unlock()

	wsPeer, isWSPeer := p.(*WSPeer)

	for _, topicName := range topics {
		topic, exists := s.topics[topicName]
		if !exists {
			topic = NewTopic(topicName, s.Config)
			s.topics[topicName] = topic
		}

		topic.peersMu.Lock()
		topic.peers[p.ID()] = p
		topic.peersMu.Unlock()

		slog.Info("peer added to topic", "peer_id", p.ID(), "topic", topicName)

		// For non-consumer group subscribers only
		if !isWSPeer || wsPeer.consumerGroup == nil {
			// Send all historical messages
			for partitionID, partition := range topic.partitions {
				size := partition.store.Len()
				if size > 0 {
					messages, err := partition.store.GetRange(0, size-1)
					if err != nil {
						slog.Error("failed to fetch stored messages", "err", err)
						continue
					}

					for _, msg := range messages {
						wsMsg := WSMessage{
							Action:    "message",
							Topics:    []string{topicName},
							Data:      string(msg.Data),
							Success:   true,
							Partition: partitionID,
							Offset:    msg.Offset,
						}

						if err := p.Send(wsMsg); err != nil {
							slog.Error("failed to send historical message",
								"peer_id", p.ID(),
								"topic", topicName,
								"err", err)
						}
					}
				}
			}
		}
	}
}

func (s *Server) AddConn(p Peer) {
	s.peersMu.Lock()
	s.peers[p.ID()] = p
	s.peersMu.Unlock()

	slog.Info("added new connection", "peer", p.ID())
}

func (s *Server) RemoveConn(p Peer) {
	s.peersMu.Lock()
	delete(s.peers, p.ID())
	s.peersMu.Unlock()

	s.topicsMu.RLock()
	for _, topic := range s.topics {
		topic.peersMu.Lock()
		delete(topic.peers, p.ID())
		topic.peersMu.Unlock()
	}
	s.topicsMu.RUnlock()

	slog.Info("removed connection", "peer", p.ID())
}

func (s *Server) RemovePeerFromTopics(p Peer, topics ...string) {
	s.topicsMu.Lock()
	defer s.topicsMu.Unlock()

	for _, topicName := range topics {
		if topic, exists := s.topics[topicName]; exists {
			topic.peersMu.Lock()
			delete(topic.peers, p.ID())
			topic.peersMu.Unlock()
		}
	}
}
