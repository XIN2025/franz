# Franz: A Lightweight Data Streaming Service

## Overview

Franz is a lightweight data streaming service built in Go, facilitating real-time message processing through a publish-subscribe model. It enables multiple consumers to subscribe to topics and process messages from assigned partitions, ensuring high concurrency and efficient message distribution. This makes Franz suitable for applications requiring real-time data processing.

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
- [Usage](#usage)
  - [Starting the Service](#starting-the-service)
  - [Publishing Messages](#publishing-messages)
  - [Adding Consumers](#adding-consumers)
  - [Message Distribution](#message-distribution)
  - [Rebalancing Partitions](#rebalancing-partitions)
  - [Historical Message Retrieval](#historical-message-retrieval)
- [Development](#development)
  - [Running Tests](#running-tests)
  - [Makefile Commands](#makefile-commands)
- [Logging](#logging)
- [Contributing](#contributing)
- [Acknowledgements](#acknowledgements)

## Features

- **Real-Time Message Distribution**: Ensures low latency by distributing messages to consumers as they arrive.
- **Dynamic Membership Management**: Supports dynamic addition and removal of consumers with automatic partition rebalancing.
- **Partitioned Data Handling**: Allows parallel processing by partitioning each topic.
- **WebSocket Communication**: Utilizes WebSocket connections for real-time server-consumer communication.
- **Historical Message Retrieval**: Enables newly joined consumers to receive historical messages from their assigned partitions.

## Getting Started

### Prerequisites

- [Go](https://golang.org/dl/) (version 1.16 or later)
- A working Go environment

### Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/yourusername/gstream.git
   cd gstream
   ```

2. **Build the project**:
   ```bash
   make build
   ```

3. **Run the service**:
   ```bash
   make run
   ```

### Configuration

Customize the server configuration by modifying the `DefaultConfig()` function in the main application file. Adjust parameters such as port numbers and buffer sizes as needed.

## Usage

### Starting the Service

After running the service, it listens for incoming WebSocket connections from consumers. Consumers can join by sending a join request specifying the topic and consumer group.

### Publishing Messages

Publish messages to a topic using an HTTP POST request:

```bash
curl -X POST http://localhost:3000/publish/mytopic -d "Your message here"
```

If the specified topic doesn't exist, Franz will automatically create it.

### Adding Consumers

Consumers can connect to the service using WebSocket and subscribe to topics. Upon joining, they receive notifications about their assigned partitions.

**Example using a WebSocket client**:

1. Connect to the WebSocket server:
   ```bash
   wscat -c ws://localhost:4000
   ```

2. Subscribe to a topic:
   ```json
   {
     "action": "subscribe",
     "topics": ["mytopic"],
     "consumer_group": "group1"
   }
   ```

### Message Distribution

Franz distributes messages to consumers based on partition ownership. Each consumer receives messages from their assigned partitions.

### Rebalancing Partitions

When a consumer joins or leaves, Franz automatically rebalances partitions among active consumers to ensure even workload distribution.

### Historical Message Retrieval

Newly joined consumers receive historical messages stored in their assigned partitions, ensuring they don't miss any data.

## Development

### Running Tests

To run tests or a test publisher:

```bash
make test
```

### Makefile Commands

- **build**: Compiles the Go code and outputs an executable in the `bin` directory.
- **run**: Builds and runs the application.
- **test**: Runs tests or a test publisher.
- **clean**: Removes the compiled binary from the `bin` directory.

## Logging

Franz uses structured logging via `log/slog` for tracking events and errors. Adjust logging levels based on your requirements.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for suggestions or improvements.


## Acknowledgements

- Inspired by messaging systems like Kafka.
- Thanks to the Go community for valuable resources and support.

---

**Note**: Replace placeholder URLs and paths (e.g., `https://github.com/yourusername/gstream.git`, `assets/logo.png`) with actual references relevant to your project.
 
