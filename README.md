# Switch - Feature Flag Management System

This is a monorepo containing both the backend and frontend components of the Switch feature flag management system.

## Project Structure

```
.
├── backend/         # Go backend service
│   ├── internal/   # Internal Go packages
│   ├── scripts/    # Backend-related scripts
│   ├── main.go     # Backend entry point
│   └── go.mod      # Go module definition
│
└── frontend/       # Frontend UI (to be implemented)
```

## Backend

The backend is a Go service that provides a distributed feature flag management system using Raft consensus. It uses BadgerDB for storage and provides a REST API for managing feature flags.

See the [backend README](./backend/README.md) for more details.

## Frontend

The frontend will be a web-based UI for managing feature flags. It will communicate with the backend service via REST API.

See the [frontend README](./frontend/README.md) for more details.

## Development

### Prerequisites

- Go 1.21 or later
- Node.js 18 or later (for frontend development)

### Getting Started

1. Clone the repository
2. Follow the setup instructions in the backend and frontend directories 