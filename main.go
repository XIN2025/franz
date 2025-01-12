package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := DefaultConfig()

	server, err := NewServer(cfg)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	if err := server.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	if err := server.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
