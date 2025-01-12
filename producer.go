package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Producer interface {
	Start() error
	Stop() error
}

type HTTPProducer struct {
	listenAddr string
	msgCh      chan Message
	server     *http.Server
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewHTTPProducer(listenAddr string, bufferSize int) *HTTPProducer {
	return &HTTPProducer{
		listenAddr: listenAddr,
		msgCh:      make(chan Message, bufferSize),
		stopCh:     make(chan struct{}),
	}
}

func (p *HTTPProducer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topic := r.URL.Path[len("/publish/"):]
	if topic == "" {
		http.Error(w, "topic required", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	msg := Message{
		Topic:     topic,
		Data:      body,
		Timestamp: time.Now(),
	}

	select {
	case p.msgCh <- msg:
		slog.Info("message sent to channel",
			"topic", topic,
			"data", string(body))
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"success":true, "topic":"%s"}`, topic)
	default:
		http.Error(w, "buffer full", http.StatusServiceUnavailable)
	}
}

func (p *HTTPProducer) Start() error {
	p.server = &http.Server{
		Addr:    p.listenAddr,
		Handler: p,
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		slog.Info("http producer started", "addr", p.listenAddr)
		if err := p.server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
		}
	}()

	return nil
}

func (p *HTTPProducer) Stop() error {
	close(p.stopCh)
	if err := p.server.Close(); err != nil {
		return fmt.Errorf("failed to close http server: %w", err)
	}
	p.wg.Wait()
	return nil
}
