# Switch - A Distributed Feature Flag System

Switch is a high-performance, distributed feature flag system built in Go. It provides a simple HTTP API for managing feature flags with support for dynamic evaluation using CEL (Common Expression Language).

## Features

- High availability through Raft consensus
- Low latency reads with BadgerDB
- Eventually consistent writes
- Dynamic flag evaluation using CEL expressions
- Simple HTTP API
- Horizontally scalable
- Pre-warming support to avoid first-request latency

## Architecture

The system consists of the following components:

- **BadgerDB**: Fast key-value store for flag data
- **Raft**: Consensus protocol for distributed state management
- **CEL**: Expression evaluation for dynamic flags
- **HTTP API**: Simple REST interface for flag management

## API

### Get Flag Value

```
GET /<store>/<key>?context=value1&other=value2
```

Returns the flag value. If the flag has an associated CEL expression, it will be evaluated using the provided context parameters.

### Set Flag Value

```
PUT /<store>/<key>
```

Request body:
```json
{
  "value": <any-json-value>,
  "expression": "optional CEL expression"
}
```

Sets a flag value and optionally associates a CEL expression for dynamic evaluation.

## CEL Expressions

Flags can be made dynamic by adding CEL expressions. The expression has access to:

- `key`: The flag key being evaluated
- `context`: Map of context values from query parameters

Example expression:
```
context.region == "us-west" && context.environment == "prod"
```

## Running the Server

```bash
# Start the first node (bootstrap)
./switch --node-id=node1 --http-addr=0.0.0.0:9990 --raft-addr=0.0.0.0:9991 --raft-advertise-addr=127.0.0.1:9991 --raft-dir=data/node1 --bootstrap --pre-warm

# Join additional nodes
./switch --node-id=node2 --http-addr=0.0.0.0:9992 --raft-addr=0.0.0.0:9993 --raft-advertise-addr=127.0.0.1:9993 --raft-dir=data/node2 --join=127.0.0.1:9991
```

### Command Line Options

- `--node-id`: Unique identifier for this node (required)
- `--http-addr`: HTTP API address (default: ":8080")
- `--raft-addr`: Raft communication address (default: ":8081")
- `--raft-dir`: Raft data directory (default: "data/raft")
- `--join`: Address of another node to join
- `--bootstrap`: Bootstrap the cluster with this node
- `--pre-warm`: Pre-warm the CEL cache on startup (default: false)

## Example Usage

```bash
# Set a simple flag
curl -X PUT localhost:8080/myapp/feature1 -d '{"value": true}'

# Set a dynamic flag with CEL expression
curl -X PUT localhost:8080/myapp/feature2 -d '{
  "value": true,
  "expression": "context.user_type == \"premium\" && context.region == \"us-west\""
}'

# Get flag value with context
curl "localhost:8080/myapp/feature2?user_type=premium&region=us-west"
```

## Pre-warming

The `--pre-warm` option can be used to pre-compile all CEL expressions during server startup. This eliminates the first-request latency penalty for flags with CEL expressions. Pre-warming is particularly useful in production environments where consistent low latency is important.

## Building

```bash
go build -o switch
```

## Requirements

- Go 1.24 or later
- Linux/macOS (Windows support untested) 