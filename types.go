// package main

// type WSMessage struct {
// 	Action  string   `json:"action"`
// 	Topics  []string `json:"topics,omitempty"`
// 	Data    string   `json:"data,omitempty"`
// 	Success bool     `json:"success"`
// 	Error   string   `json:"error,omitempty"`
// }

package main

import "time"

type Message struct {
	Topic     string
	Data      []byte
	Timestamp time.Time
	Partition int
	Offset    int64
}

type WSMessage struct {
	Action        string   `json:"action"`
	Topics        []string `json:"topics,omitempty"`
	Data          string   `json:"data,omitempty"`     // Keep for backward compatibility or optional text data
	DataRaw       []byte   `json:"data_raw,omitempty"` // New field for raw byte data
	Success       bool     `json:"success"`
	Error         string   `json:"error,omitempty"`
	ConsumerGroup string   `json:"consumer_group,omitempty"`
	Partition     int      `json:"partition,omitempty"`
	Offset        int64    `json:"offset,omitempty"`
}
