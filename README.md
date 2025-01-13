# README for Gstream: A Data Streaming Service

## Overview

Gstream is a lightweight data streaming service built in Go that facilitates real-time message processing through a publish-subscribe model. It allows multiple consumers to subscribe to topics and process messages from partitions assigned to them. This service is designed for high concurrency and efficient message distribution, making it suitable for applications requiring real-time data processing.

## Features

- **Real-Time Message Distribution**: Distributes messages to consumers as they arrive, ensuring low latency.
- **Dynamic Membership Management**: Supports adding and removing consumer members dynamically, with automatic rebalancing of partition assignments.
- **Partitioned Data Handling**: Each topic can have multiple partitions, allowing for parallel processing of messages.
- **WebSocket Communication**: Utilizes WebSocket connections for real-time communication between the server and consumers.
- **Historical Message Retrieval**: Newly joined consumers can receive historical messages for their assigned partitions.

## Getting Started

### Prerequisites

- Go (version 1.16 or later) installed on your machine.
- A working Go environment set up.

### Installation

1. Clone the repository:
```
git clone https://github.com/yourusername/gstream.git
```
```
cd gstream
```
2. Build the project:
