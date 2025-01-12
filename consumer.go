package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Consumer interface {
	Start() error
	Stop() error
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // For testing purposes
	},
	// HandshakeTimeout:  10 * time.Second,
	// ReadBufferSize:    1024,
	// WriteBufferSize:   1024,
	// EnableCompression: true,
}

type wsConsumer struct {
	ListenAddr  string
	server      *Server
	httpServer  *http.Server
	stopCh      chan struct{}
	wg          sync.WaitGroup
	activeConns map[string]Peer
	connMu      sync.RWMutex
}

func NewWSConsumer(ListenAddr string, server *Server) *wsConsumer {
	return &wsConsumer{
		ListenAddr:  ListenAddr,
		server:      server,
		stopCh:      make(chan struct{}),
		activeConns: make(map[string]Peer),
	}
}

func (ws *wsConsumer) Start() error {
	slog.Info("websocket consumer starting", "port", ws.ListenAddr)

	ws.httpServer = &http.Server{
		Addr:    ws.ListenAddr,
		Handler: ws,
	}

	ws.wg.Add(1)
	go func() {
		defer ws.wg.Done()
		if err := ws.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("websocket server error", "err", err)
		}
	}()

	ws.wg.Add(1)
	go ws.monitorStopChannel()

	return nil
}

func (ws *wsConsumer) monitorStopChannel() {
	defer ws.wg.Done()
	<-ws.stopCh
	slog.Info("websocket consumer received stop signal")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ws.httpServer.Shutdown(ctx); err != nil {
		slog.Error("error shutting down http server", "err", err)
	}

	ws.closeAllConnections()
}

func (ws *wsConsumer) closeAllConnections() {
	ws.connMu.Lock()
	defer ws.connMu.Unlock()

	for id, peer := range ws.activeConns {
		slog.Info("closing connection", "peer_id", id)
		if err := peer.Close(); err != nil {
			slog.Error("error closing peer connection", "peer_id", id, "err", err)
		}
		delete(ws.activeConns, id)
	}
}

func (ws *wsConsumer) Stop() error {
	slog.Info("stopping websocket consumer")
	close(ws.stopCh)

	done := make(chan struct{})
	go func() {
		ws.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("websocket consumer stopped gracefully")
	case <-time.After(10 * time.Second):
		slog.Error("timeout waiting for websocket consumer to stop")
	}

	return nil
}

func (ws *wsConsumer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "err", err)
		return
	}

	peer := NewWSPeer(conn, ws.server)

	ws.connMu.Lock()
	ws.activeConns[peer.ID()] = peer
	ws.connMu.Unlock()
	go func() {
		<-peer.stopCh

		ws.connMu.Lock()
		delete(ws.activeConns, peer.ID())
		ws.connMu.Unlock()

		ws.server.RemoveConn(peer)
	}()

	ws.server.AddConn(peer)
}

func (ws *wsConsumer) ActiveConnectionCount() int {
	ws.connMu.RLock()
	defer ws.connMu.RUnlock()
	return len(ws.activeConns)
}
