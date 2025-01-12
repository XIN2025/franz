package main

// import (
// 	"log"
// 	"os"
// 	"os/signal"

// 	"github.com/gorilla/websocket"
// )

// type WSMessage struct {
// 	Action  string   `json:"action"`
// 	Topics  []string `json:"topics,omitempty"`
// 	Data    string   `json:"data,omitempty"`
// 	Success bool     `json:"success"`
// 	Error   string   `json:"error,omitempty"`
// }

// func main() {
// 	// Connect to WebSocket server
// 	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:4000", nil)
// 	if err != nil {
// 		log.Fatalf("failed to connect to WebSocket: %v", err)
// 	}
// 	defer conn.Close()

// 	// Subscribe to a topic
// 	subscribeMsg := WSMessage{
// 		Action: "subscribe",
// 		Topics: []string{"test"},
// 	}

// 	if err := conn.WriteJSON(subscribeMsg); err != nil {
// 		log.Fatalf("failed to send subscription: %v", err)
// 	}

// 	// Handle interrupt signal
// 	interrupt := make(chan os.Signal, 1)
// 	signal.Notify(interrupt, os.Interrupt)

// 	// Message handling loop
// 	for {
// 		select {
// 		case <-interrupt:
// 			// Cleanly close the connection
// 			err := conn.WriteMessage(
// 				websocket.CloseMessage,
// 				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
// 			)
// 			if err != nil {
// 				log.Printf("error sending close message: %v", err)
// 			}
// 			return
// 		default:
// 			var msg WSMessage
// 			err := conn.ReadJSON(&msg)
// 			if err != nil {
// 				log.Printf("error reading message: %v", err)
// 				return
// 			}

// 			if msg.Error != "" {
// 				log.Printf("received error: %s", msg.Error)
// 				continue
// 			}

// 			switch msg.Action {
// 			case "subscribed":
// 				log.Printf("Successfully subscribed to topics: %v", msg.Topics)
// 			case "message":
// 				log.Printf("Received message: %s", msg.Data)
// 			default:
// 				log.Printf("Received message with action: %s", msg.Action)
// 			}
// 		}
// 	}
// }
