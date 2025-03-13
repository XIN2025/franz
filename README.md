#Franz: A Data Streaming Service

## Overview

Franz is a lightweight data streaming service built in Go that facilitates real-time message processing through a publish-subscribe model. It allows multiple consumers to subscribe to topics and process messages from partitions assigned to them. This service is designed for high concurrency and efficient message distribution, making it suitable for applications requiring real-time data processing.

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
```
make build
 ```

3. Run the service:
```
make run
 ```

### Running Tests

To run tests or a test publisher, use the following command:
```
make test
 ```

### Cleaning Up

To clean up the built binaries, use:
```
make clean
```


## Usage

### Starting the Service

Once the service is running, it listens for incoming WebSocket connections from consumers. Consumers can join by sending a join request with the specified topic and consumer group.

### Adding Consumers

Consumers can connect to the service using WebSocket and subscribe to topics. Upon joining, they will receive notifications about their assigned partitions.

### Message Distribution

Messages are published to topics, and the service will distribute these messages to the appropriate consumer based on partition ownership.

### Rebalancing Partitions

When a consumer joins or leaves, the service automatically rebalances partitions among active consumers to ensure even workload distribution.

## Makefile Explanation

The provided Makefile contains several targets for building and managing the Franz application:

- **build**: Compiles the Go code and outputs an executable in the `bin` directory.
- **run**: Builds the application and then runs it.
- **testp**: Runs a test publisher from the specified directory.
- **clean**: Removes the compiled binary from the `bin` directory.

## Example Configuration

You can customize the server configuration by modifying the `DefaultConfig()` function in your main application file. Adjust parameters such as port numbers, buffer sizes, and other settings as needed.

## Logging

The service uses structured logging via `log/slog` for tracking events and errors. You can adjust logging levels based on your requirements.

## Contributing

Contributions are welcome! If you have suggestions or improvements, please open an issue or submit a pull request.



