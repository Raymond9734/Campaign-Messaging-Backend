# SMSLeopard Backend Challenge

A production-grade backend service for managing SMS and WhatsApp campaigns, built with Go.

## Overview

This project implements a campaign dispatch service that:

- Manages campaigns with personalized message templates
- Queues messages for asynchronous delivery via Redis
- Processes messages with a worker using a mock sender (92% success rate)
- Supports retry logic (max 3 attempts)
- Provides RESTful API endpoints with pagination and filtering

## Architecture

```txt
┌─────────────┐         ┌──────────┐         ┌────────────┐
│   API       │────────▶│  Redis   │────────▶│   Worker   │
│   Server    │         │  Queue   │         │            │
└──────┬──────┘         └──────────┘         └──────┬─────┘
       │                                             │
       │                                             │
       └────────────────┬────────────────────────────┘
                        │
                   ┌────▼─────┐
                   │PostgreSQL│
                   └──────────┘
```

## Technology Stack

- **Language**: Go 1.24
- **Database**: PostgreSQL 15
- **Queue**: Redis 7
- **Router**: Chi v5
- **Logging**: log/slog (structured JSON logging)
- **Containerization**: Docker + Docker Compose

## Project Structure

```txt
.
├── cmd/
│   ├── api/          # API server entrypoint
│   └── worker/       # Worker entrypoint
├── internal/
│   ├── config/       # Configuration management
│   ├── db/           # Database connection
│   ├── handler/      # HTTP handlers
│   ├── models/       # Domain models
│   ├── queue/        # Redis queue client
│   ├── repository/   # Data access layer
│   ├── service/      # Business logic
│   └── worker/       # Worker processor & mock sender
├── migrations/       # Database migrations
├── docker-compose.yml
├── Dockerfile.api
├── Dockerfile.worker
└── Makefile
```

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.24+ (for local development)
- Make (optional, for convenience commands)

### Running with Docker (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/Raymond9734/smsleopard-backend-challenge.git
cd smsleopard-backend-challenge

# 2. Setup environment variables
make setup
# OR manually: cp .env.example .env

# 3. Start all services (PostgreSQL, Redis, API, Worker)
make docker-up

# 4. Verify services are running
curl http://localhost:8080/health

# 5. View logs
make docker-logs

# 6. Stop services
make docker-down
```

The database migrations and seed data run automatically on first startup.

### Running Locally (Development)

```bash
# 1. Setup environment
make setup

# 2. Start PostgreSQL and Redis (via Docker)
docker-compose up -d postgres redis

# 3. Run migrations
make migrate-up

# 4. Run API server (terminal 1)
make run-api

# 5. Run worker (terminal 2)
make run-worker
```

## API Endpoints

### Health Check

```http
GET /health
```

### Campaign Endpoints

#### Create Campaign

```http
POST /campaigns
Content-Type: application/json

{
  "name": "Summer Sale 2025",
  "channel": "sms",
  "base_template": "Hi {first_name}, check out {preferred_product} in {location}!",
  "scheduled_at": "2025-06-01T10:00:00Z"  // optional
}
```

#### List Campaigns

```http
GET /campaigns?page=1&page_size=20&channel=sms&status=draft
```

#### Get Campaign Details

```http
GET /campaigns/{id}
```

**Response includes statistics:**

```json
{
  "id": 1,
  "name": "Summer Sale 2025",
  "stats": {
    "total": 100,
    "pending": 45,
    "sent": 50,
    "failed": 5
  }
}
```

#### Send Campaign

```http
POST /campaigns/{id}/send
Content-Type: application/json

{
  "customer_ids": [1, 2, 3, 4, 5]
}
```

#### Personalized Preview

```http
POST /campaigns/{id}/personalized-preview
Content-Type: application/json

{
  "customer_id": 123,
  "override_template": "Hi {first_name}!"  // optional
}
```

## Template System

### How Templates Work

Templates use placeholder syntax: `{field_name}`

**Available placeholders:**

- `{first_name}` - Customer first name
- `{last_name}` - Customer last name
- `{location}` - Customer location
- `{preferred_product}` - Customer's preferred product
- `{phone}` - Customer phone number

**Example:**

```md
Template: "Hi {first_name}, check out {preferred_product} in {location}!"
Customer: {first_name: "Alice", preferred_product: "Running Shoes", location: "Nairobi"}
Result: "Hi Alice, check out Running Shoes in Nairobi!"
```

### Missing Field Handling

If a customer field is empty or missing, it's replaced with an **empty string**:

```md
Template: "Hi {first_name} {last_name}"
Customer: {first_name: "Alice", last_name: ""}
Result: "Hi Alice "
```

This allows campaigns to proceed even with incomplete customer data.

## Mock Sender Behavior

The worker uses a **mock sender** that simulates real message delivery:

- **Success Rate**: 92% (configurable)
- **Network Latency**: 50-200ms delay
- **Failure Mode**: Random "simulated network error"
- **Purpose**: Test retry logic and error handling

## Retry Logic

Messages that fail to send are automatically retried:

1. **First attempt fails** → Status: "failed", retry_count: 1
2. **Second attempt fails** → Status: "failed", retry_count: 2
3. **Third attempt fails** → Status: "failed" (permanently), retry_count: 3

Maximum retries: **3 attempts** (configurable via `MAX_RETRY_COUNT`)

**Note**: In this implementation, failed messages remain in the database. In production, consider implementing:

- Exponential backoff for retries
- Dead letter queue (DLQ) for permanently failed messages
- Automatic re-queuing with delays

## Queue Choice: Redis

**Why Redis?**

- **Simple**: No complex broker setup (vs RabbitMQ)
- **Fast**: In-memory operations
- **Reliable**: Atomic LPUSH/BRPOP operations
- **Observable**: Monitor queue length with `LLEN campaign_sends`
- **Battle-tested**: Industry-standard for job queues

**Queue Pattern:**

- API publishes jobs: `LPUSH campaign_sends <job_json>`
- Worker consumes jobs: `BRPOP campaign_sends 1` (blocking)
- FIFO ordering preserved

## Database Schema

### Key Tables

#### Customers

- Stores customer information for targeting
- Indexed on `phone` for fast lookups

#### campaigns

- Campaign metadata and template
- Indexed on `status`, `channel`, `id` for filtering/pagination

#### outbound_messages

- Individual messages to be sent
- Composite index on `(campaign_id, status)` for stats queries
- Index on `(status, created_at)` for worker queue processing

See `migrations/001_initial_schema_up.sql` for complete schema.

## Configuration

All configuration via environment variables (see `.env.example`):

| Variable          | Description          | Default                  |
| ----------------- | -------------------- | ------------------------ |
| `DB_HOST`         | PostgreSQL host      | localhost                |
| `DB_PORT`         | PostgreSQL port      | 5432                     |
| `DB_USER`         | Database user        | smsleopard               |
| `DB_PASSWORD`     | Database password    | smsleopard               |
| `DB_NAME`         | Database name        | smsleopard               |
| `REDIS_URL`       | Redis connection URL | redis://localhost:6379/0 |
| `QUEUE_NAME`      | Queue name           | campaign_sends           |
| `API_PORT`        | API server port      | 8080                     |
| `MAX_RETRY_COUNT` | Max send attempts    | 3                        |

## Makefile Commands

```bash
make help              # Show all available commands
make setup             # Create .env from .env.example
make build             # Build API and worker binaries
make run-api           # Run API server locally
make run-worker        # Run worker locally
make docker-up         # Start all services with Docker
make docker-down       # Stop all services
make docker-logs       # View Docker logs
make migrate-up        # Run database migrations
make migrate-down      # Rollback migrations
```

## Testing the System

### 1. Create a Campaign

```bash
curl -X POST http://localhost:8080/campaigns \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Campaign",
    "channel": "sms",
    "base_template": "Hi {first_name}!"
  }'
```

### 2. Send Campaign to Customers

```bash
curl -X POST http://localhost:8080/campaigns/1/send \
  -H "Content-Type: application/json" \
  -d '{
    "customer_ids": [1, 2, 3, 4, 5]
  }'
```

### 3. Check Campaign Stats

```bash
curl http://localhost:8080/campaigns/1
```

### 4. Monitor Worker Logs

```bash
docker-compose logs -f worker
```

## Extra Feature: Idempotency

### What Was Implemented

**Application-level idempotency** to prevent duplicate campaign sends if the API is called multiple times.

### How It Works

The system uses campaign status as a state machine to enforce idempotency:

```txt
POST /campaigns/1/send (First Call)
  ↓
Status: "draft" → Process messages → Status: "sending"
✓ Returns: {"campaign_id": 1, "messages_queued": 5, "status": "sending"}

POST /campaigns/1/send (Second Call)
  ↓
Status: "sending" → CanBeSent() check fails
✗ Returns: 409 Conflict - "campaign already processed"
```

**Implementation Details:**

- `CanBeSent()` method checks if status is "draft" or "scheduled"
- Once status changes to "sending", "sent", or "failed", the campaign cannot be sent again
- Returns clear 409 Conflict error with explanation
- Logs idempotency failures for auditing

### Why Idempotency Is Critical

**Real-World Scenarios:**

1. **Network Timeouts**: Client retries request after timeout, thinking it failed
2. **User Error**: User clicks "Send" button multiple times
3. **Load Balancer Retries**: Infrastructure-level request duplication
4. **Race Conditions**: Multiple concurrent requests to send same campaign

**Without Idempotency:**

- Customers receive duplicate messages (poor UX, cost increase)
- Database bloat with duplicate message records
- Queue flooded with duplicate jobs
- Wasted SMS/WhatsApp credits

**With Idempotency:**

- Same campaign can only be sent once
- Graceful error messages guide users
- Audit trail of duplicate attempts
- Cost protection and customer experience

### Trade-offs

**Chosen Approach (Status-Based):**

- **Pros**: Simple, no schema changes, easy to understand
- **Cons**: Campaign can never be resent (even if desired)

**Alternative (Not Implemented):**

- Idempotency keys in request headers
- Allow resend with explicit "force" flag
- Time-based idempotency windows

For this use case, preventing duplicate sends is more important than allowing resends. To resend, create a new campaign.

### Scheduled Campaigns Note

The system supports `scheduled_at` field for campaigns:

- Campaigns with `scheduled_at` are marked as "scheduled" status
- **Manual trigger required**: Call `/campaigns/{id}/send` when ready
- **Not implemented**: Automatic background dispatch at scheduled time

---

## Design Decisions

### Why Chi Router?

- Lightweight, idiomatic Go
- Path parameters support (`/campaigns/{id}`)
- Middleware-friendly
- Zero external dependencies beyond stdlib patterns

### Why Mock Sender?

- Challenge focuses on backend architecture
- No real SMS API keys required
- Easy to test failure scenarios
- Simple to replace with real provider (interface-based)

### Why Batch Message Creation?

- Reduces database round trips (1 transaction vs N queries)
- Improves throughput for large campaigns
- Maintains data consistency

### Why Stable Pagination?

- `ORDER BY id DESC` ensures consistent results
- No duplicates or missing records between pages
- Works even as new campaigns are created

## Time Spent & Tools Used

**Total Time**: ~6-8 hours

**AI Tools Used**:

- Claude Code for implementation guidance
- GitHub Copilot for boilerplate code generation
- ChatGPT for Go best practices research

## License

This is a take-home coding challenge. Code may be used for evaluation purposes.

## Author

## Raymond Madara

### Built with using Go and production best practices
