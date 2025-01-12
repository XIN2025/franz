package main

import (
	"bytes"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

// Message represents a message received by the test client
type Message struct {
	Action string
	Topics []string
	Data   string
}

// TestClient represents a test client
type TestClient struct {
	conn             *websocket.Conn
	subscriptions    []string
	receivedMessages []Message
}

// NewTestClient creates a new test client
func NewTestClient(url string) (*TestClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &TestClient{conn: conn}, nil
}

// Close closes the test client connection
func (c *TestClient) Close() {
	c.conn.Close()
}

// Start starts the test client (placeholder for actual logic)
func (c *TestClient) Start() error {
	go c.listenForMessages()
	return nil
}

// Subscribe subscribes the test client to topics
func (c *TestClient) Subscribe(topics ...string) error {
	for _, topic := range topics {
		c.subscriptions = append(c.subscriptions, topic)
		c.receivedMessages = append(c.receivedMessages, Message{Action: "subscribed", Topics: []string{topic}})
	}
	return nil
}

// Unsubscribe unsubscribes the test client from topics
func (c *TestClient) Unsubscribe(topics ...string) error {
	for _, topic := range topics {
		for i, subscribedTopic := range c.subscriptions {
			if subscribedTopic == topic {
				c.subscriptions = append(c.subscriptions[:i], c.subscriptions[i+1:]...)
				c.receivedMessages = append(c.receivedMessages, Message{Action: "unsubscribed", Topics: []string{topic}})
				break
			}
		}
	}
	return nil
}

// GetSubscriptions returns the current subscriptions of the test client
func (c *TestClient) GetSubscriptions() []string {
	return c.subscriptions
}

// GetReceivedMessages returns the received messages of the test client
func (c *TestClient) GetReceivedMessages() []Message {
	return c.receivedMessages
}

// listenForMessages listens for incoming messages from the WebSocket connection
func (c *TestClient) listenForMessages() {
	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}
		c.receivedMessages = append(c.receivedMessages, msg)
	}
}

func main() {
	log.Println("Creating test clients...")
	client1, err := NewTestClient("ws://localhost:4000")
	if err != nil {
		log.Fatalf("Failed to create client 1: %v", err)
	}
	defer client1.Close()

	client2, err := NewTestClient("ws://localhost:4000")
	if err != nil {
		log.Fatalf("Failed to create client 2: %v", err)
	}
	defer client2.Close()

	log.Println("Starting clients...")
	if err := client1.Start(); err != nil {
		log.Fatalf("Failed to start client 1: %v", err)
	}
	if err := client2.Start(); err != nil {
		log.Fatalf("Failed to start client 2: %v", err)
	}

	log.Println("Subscribing to topics...")
	if err := client1.Subscribe("topic1", "topic2"); err != nil {
		log.Printf("Failed to subscribe client 1: %v", err)
	}
	if err := client2.Subscribe("topic2"); err != nil {
		log.Printf("Failed to subscribe client 2: %v", err)
	}

	log.Println("Publishing messages...")
	publishMessage("topic1", "Hello from topic 1!")
	publishMessage("topic2", "Hello from topic 2!")

	time.Sleep(2 * time.Second)

	printClientStats("Client 1", client1)
	printClientStats("Client 2", client2)

	log.Println("\nUnsubscribing client1 from topic1...")
	if err := client1.Unsubscribe("topic1"); err != nil {
		log.Printf("Failed to unsubscribe: %v", err)
	}

	log.Println("\nPublishing more messages...")
	publishMessage("topic1", "This message shouldn't reach client1")
	publishMessage("topic2", "This message should reach both clients")

	time.Sleep(2 * time.Second)

	log.Println("\nFinal Statistics:")
	printClientStats("Client 1", client1)
	printClientStats("Client 2", client2)
}

// Helper function to publish messages via HTTP
func publishMessage(topic, message string) {
	url := fmt.Sprintf("http://localhost:3000/topic/%s", topic)
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(message))
	if err != nil {
		log.Printf("Failed to publish message: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Unexpected status code: %d", resp.StatusCode)
		return
	}

	log.Printf("Published message to %s: %s", topic, message)
}

// Helper function to print client statistics
func printClientStats(clientName string, client *TestClient) {
	log.Printf("\n=== %s Statistics ===", clientName)
	log.Printf("Subscribed Topics: %v", client.GetSubscriptions())

	messages := client.GetReceivedMessages()
	log.Printf("Total Messages Received: %d", len(messages))

	log.Println("Message History:")
	for i, msg := range messages {
		switch msg.Action {
		case "subscribed":
			log.Printf("%d. ✅ Subscribed to: %v", i+1, msg.Topics)
		case "unsubscribed":
			log.Printf("%d. ❌ Unsubscribed from: %v", i+1, msg.Topics)
		case "message":
			log.Printf("%d. 📨 Received: %s", i+1, msg.Data)
		default:
			log.Printf("%d. 📝 Action: %s", i+1, msg.Action)
		}
	}
}
