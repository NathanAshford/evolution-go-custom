# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Evolution GO is a WhatsApp API gateway built in Go that provides RESTful APIs for WhatsApp messaging. It uses the whatsmeow library to interface with WhatsApp Web protocol and follows a clean architecture pattern with clear separation of concerns.

## Essential Commands

### Development
```bash
# Run in development mode with hot-reload
make dev

# Watch for changes and auto-restart (requires entr)
make watch

# Quick development setup
make quick-start
```

### Build & Run
```bash
# Build for Linux/Docker
make build

# Build for current platform
make build-local

# Build and run locally
make run
```

### Testing & Quality
```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Lint code (fmt + vet)
make lint

# Full check (deps + lint + test)
make check

# Watch and auto-run tests on changes
make watch-test
```

### Docker
```bash
# Build Docker image
make docker-build

# Run Docker container
make docker-run

# Run in development mode
make docker-run-dev
```

### Documentation
```bash
# Generate Swagger API docs
make swagger
```

## Architecture

### Package Structure
The codebase follows a **3-layer architecture** for each module:
- **Handler** (`handler.go`) - HTTP request/response handling
- **Service** (`service.go`) - Business logic implementation
- **Repository** (`repository.go`) - Data access layer

### Core Packages
- `pkg/instance/` - WhatsApp instance lifecycle management
- `pkg/whatsmeow/` - WhatsApp Web client wrapper around whatsmeow library
- `pkg/message/` - Message handling, storage, and retrieval
- `pkg/sendMessage/` - Message sending service with media support
- `pkg/events/` - Event producers (AMQP, NATS, Webhook, WebSocket)
- `pkg/storage/` - Media storage abstraction (MinIO/local)
- `pkg/config/` - Configuration management and environment variables

### Database Architecture
- **Dual database system**:
  - `evogo_auth` - Authentication and API keys
  - `evogo_users` - User data and WhatsApp instances
- Uses GORM ORM with PostgreSQL (primary) and SQLite (fallback)
- Connection pooling with configurable limits

### Event System
Multiple event producers can be enabled simultaneously:
- **RabbitMQ** - AMQP message queuing
- **NATS** - Lightweight messaging
- **WebSocket** - Real-time client connections
- **Webhook** - HTTP callbacks to external services

## Key Development Patterns

### Adding New API Endpoints
1. Define handler in appropriate package (e.g., `pkg/message/handler.go`)
2. Implement service logic in `service.go`
3. Add repository methods in `repository.go` if database access needed
4. Register route in `cmd/evolution-go/main.go`
5. Update Swagger documentation with `make swagger`

### Working with WhatsApp Client
- Client instances are managed in `pkg/whatsmeow/`
- Each instance maintains its own database connection
- Events are propagated through the event system
- Media files are stored via the storage abstraction layer

### Environment Configuration
Required environment variables are loaded from `.env` file:
- `SERVER_PORT` - API server port (default: 4000)
- `POSTGRES_AUTH_DB` - Auth database connection string
- `POSTGRES_USERS_DB` - Users database connection string
- `GLOBAL_API_KEY` - API authentication key
- `DATABASE_SAVE_MESSAGES` - Enable message persistence

## Testing Guidelines

### Running Specific Tests
```bash
# Test specific package
go test ./pkg/message/...

# Test with verbose output
go test -v ./pkg/...

# Run specific test function
go test -run TestFunctionName ./pkg/package/
```

### Writing Tests
- Place tests in `*_test.go` files alongside implementation
- Use table-driven tests for multiple scenarios
- Mock external dependencies (WhatsApp client, database)
- Test files should cover handlers, services, and repositories

## Common Tasks

### Adding Database Migrations
1. Modify models in appropriate package
2. GORM auto-migrates on startup via `AutoMigrate()`
3. Add migration logic in `pkg/user/repository.go` or `pkg/instance/repository.go`

### Debugging WhatsApp Connection Issues
1. Set `WADEBUG=DEBUG` environment variable
2. Check logs in `logs/` directory
3. Verify instance status via `/instance/connectionState` endpoint
4. Review WebSocket events if enabled

### Implementing New Message Types
1. Add message structure in `pkg/message/model.go`
2. Implement sending logic in `pkg/sendMessage/service.go`
3. Add handler method in `pkg/sendMessage/handler.go`
4. Update Swagger docs with new endpoint

## Important Considerations

- **License Validation**: Production builds validate licenses against external API
- **Media Storage**: Configure MinIO for production; uses local storage in development
- **Connection Limits**: Default PostgreSQL pool size is 100 connections
- **Rate Limiting**: Framework supports rate limiting but not enabled by default
- **Security**: All endpoints require API key authentication via `apikey` header