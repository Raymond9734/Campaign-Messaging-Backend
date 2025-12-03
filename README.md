# SMSLeopard Backend Challenge

A backend service for managing SMS and WhatsApp campaigns, built with Go.

## Overview

This project implements a campaign dispatch service that:

- Manages campaigns with personalized message templates
- Queues messages for asynchronous delivery via Redis
- Processes messages with a worker using a mock sender (92% success rate)
- Supports retry logic (max 3 attempts)(Not fully implemented)
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

**Base URL**: `http://localhost:8080/api`

### Health Check

```http
GET /health
```

### Campaign Endpoints

#### Create Campaign

```http
POST /api/campaigns
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
GET /api/campaigns?page=1&page_size=20&channel=sms&status=draft
```

#### Get Campaign Details

```http
GET /api/campaigns/{id}
```

**Response includes statistics:**

```json
{
  "id": 1,
  "name": "Summer Sale 2025",
  "stats": {
    "total": 100,
    "pending": 45,
    "sending": 0,
    "sent": 50,
    "failed": 5
  }
}
```

**Note**: The `"sending"` field is always 0 because individual messages only have `pending`, `sent`, or `failed` statuses (no in-flight "sending" status).

#### Send Campaign

```http
POST /api/campaigns/{id}/send
Content-Type: application/json

{
  "customer_ids": [1, 2, 3, 4, 5]
}
```

**Customer Selection:**

Currently, you must manually specify `customer_ids` to target specific customers. This provides precise control over campaign recipients.

**Future Enhancements:**

In a production system, we can consider these targeting approaches:

1. **Send to All Customers**

   ```json
   {
     "target": "all"
   }
   ```

   - Fetches all customer IDs from database
   - Useful for mass announcements

2. **Segmentation Criteria**

   ```json
   {
     "segment": {
       "location": "Nairobi",
       "preferred_product": "Running Shoes"
     }
   }
   ```

   - Filter customers by attributes
   - Enables targeted marketing

3. **Dynamic Lists**

   ```json
   {
     "customer_list_id": 42
   }
   ```

   - Pre-defined customer segments
   - Reusable across campaigns

4. **Exclusion Lists**
   ```json
   {
     "customer_ids": [1, 2, 3],
     "exclude_ids": [2]
   }
   ```
   - Prevent duplicate sends
   - Honor unsubscribe requests

**Implementation Considerations:**

- **Performance**: Fetching all customers requires pagination for large datasets
- **Rate Limiting**: Throttle campaign sends to avoid overwhelming SMS providers
- **Compliance**: Track opt-outs and respect customer preferences
- **Analytics**: Log targeting criteria for campaign performance analysis

#### Personalized Preview

```http
POST /api/campaigns/{id}/personalized-preview
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

**Current Implementation (Limited):**

Messages that fail are tracked but **NOT automatically retried**:

1. **First attempt fails** → Status: "failed", retry_count: 1
2. Message remains in database with status "failed"
3. No automatic re-queuing happens

**Configuration:**

- `MAX_RETRY_COUNT=3` is configured but only tracks the retry count
- Failed messages stay in the database and require manual intervention to retry

**Why This Limitation?**

This simplified implementation focuses on:

- Tracking failure information (retry_count, last_error)
- Demonstrating the retry tracking mechanism
- Clean separation of concerns

**Production Improvements Needed:**

To achieve true automatic retries (3 attempts), implement:

1. **Automatic Re-queuing**: When a message fails with retry_count < MAX_RETRY_COUNT, publish it back to the Redis queue
2. **Exponential Backoff**: Add delays between retries (1s, 2s, 4s, 8s, etc.)
3. **Dead Letter Queue (DLQ)**: Move permanently failed messages (retry_count >= 3) to a separate queue for manual review
4. **Retry Scheduler**: Use delayed job processing or time-based workers

**Example Flow (Not Implemented):**

```md
Attempt 1 fails → retry_count: 1 → Re-queue with 1s delay
Attempt 2 fails → retry_count: 2 → Re-queue with 2s delay
Attempt 3 fails → retry_count: 3 → Move to DLQ (permanent failure)
```

## Queue Choice: Redis

**Why Redis?**

- **Simple**: No complex broker setup
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

| Variable             | Description                               | Default                  |
| -------------------- | ----------------------------------------- | ------------------------ |
| `DB_HOST`            | PostgreSQL host                           | localhost                |
| `DB_PORT`            | PostgreSQL port                           | 5432                     |
| `DB_USER`            | Database user                             | smsleopard               |
| `DB_PASSWORD`        | Database password                         | smsleopard               |
| `DB_NAME`            | Database name                             | smsleopard               |
| `REDIS_URL`          | Redis connection URL                      | redis://localhost:6379/0 |
| `QUEUE_NAME`         | Queue name                                | campaign_sends           |
| `API_PORT`           | API server port                           | 8080                     |
| `WORKER_CONCURRENCY` | Max concurrent message processing (max 5) | 5                        |
| `MAX_RETRY_COUNT`    | Max send attempts per message             | 3                        |

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

## Running Tests

**Note**: Test suites for template rendering, pagination, worker logic, and personalized preview are included in the codebase.

```bash
# Run all tests
make test

# Run tests with coverage
go test -v -cover ./...

# Run specific package tests
go test -v ./internal/service/...
go test -v ./internal/repository/...
go test -v ./internal/worker/...
```

**Test Coverage Areas**:

- Template rendering with various placeholder combinations
- Pagination with different page sizes and filters
- Worker message processing and retry logic
- Personalized preview with override templates
- Error handling and edge cases

## Assumptions Made

1. **Schema Compliance**: Database schema strictly follows the provided specification (no extra fields beyond what's specified)

2. **Customer Data**:

   - Missing or null customer fields are replaced with empty strings (not errors)
   - Allows campaigns to proceed with incomplete customer data
   - Assumes data enrichment happens gradually

3. **Message Retries**:

   - Failed messages are NOT automatically re-queued in this implementation
   - Manual intervention required to retry failed messages
   - In production, would implement exponential backoff and dead letter queue

4. **Campaign Resend**:

   - Once a campaign is sent (status: "sending"/"sent"/"failed"), it cannot be resent
   - To resend, create a new campaign with the same template
   - Idempotency prioritized over flexibility

5. **Scheduled Campaigns**:

   - Campaigns with `scheduled_at` require manual triggering via API
   - No background scheduler implemented to automatically dispatch at scheduled time
   - Simple to add with cron job or time-based worker

6. **Worker Concurrency**:

   - Supports concurrent message processing (up to 5 messages simultaneously per worker)
   - Controlled via `WORKER_CONCURRENCY` environment variable (default: 5, max: 5)
   - Uses semaphore pattern to limit concurrent goroutines
   - For higher throughput, run multiple worker instances (horizontal scaling)
   - Graceful shutdown waits for in-flight jobs to complete

7. **Stats "sending" Field**:
   - Always returns 0 because individual messages don't have "sending" status
   - Field included for specification compliance
   - Campaign-level "sending" status is separate from message-level statuses

## Testing the System

### 1. Create a Campaign

```bash
curl -X POST http://localhost:8080/api/campaigns \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Campaign",
    "channel": "sms",
    "base_template": "Hi {first_name}!"
  }'
```

**Expected Response:**

```json
{
  "id": 1,
  "name": "Test Campaign",
  "channel": "sms",
  "status": "draft",
  "base_template": "Hi {first_name}!",
  "scheduled_at": null,
  "created_at": "2025-12-03T10:00:00Z"
}
```

### 2. List Campaigns (with Filters)

```bash
# List all campaigns (paginated)
curl http://localhost:8080/api/campaigns

# Filter by status
curl http://localhost:8080/api/campaigns?status=draft

# Filter by channel
curl http://localhost:8080/api/campaigns?channel=sms

# Pagination
curl http://localhost:8080/api/campaigns?page=1&page_size=10

# Combined filters
curl "http://localhost:8080/api/campaigns?status=sent&channel=whatsapp&page=1&page_size=20"
```

**Expected Response:**

```json
{
  "data": [
    {
      "id": 1,
      "name": "Test Campaign",
      "channel": "sms",
      "status": "draft",
      "created_at": "2025-12-03T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total_count": 1,
    "total_pages": 1
  }
}
```

### 3. Get Campaign Details (with Stats)

```bash
curl http://localhost:8080/api/campaigns/1
```

**Expected Response:**

```json
{
  "id": 1,
  "name": "Test Campaign",
  "channel": "sms",
  "status": "draft",
  "base_template": "Hi {first_name}!",
  "scheduled_at": null,
  "created_at": "2025-12-03T10:00:00Z",
  "stats": {
    "total": 0,
    "pending": 0,
    "sending": 0,
    "sent": 0,
    "failed": 0
  }
}
```

### 4. Preview Personalized Message

```bash
# Preview with campaign template
curl -X POST http://localhost:8080/api/campaigns/1/personalized-preview \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": 1
  }'

# Preview with override template
curl -X POST http://localhost:8080/api/campaigns/1/personalized-preview \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": 1,
    "override_template": "Hello {first_name}, special offer for {location}!"
  }'
```

**Expected Response:**

```json
{
  "rendered_message": "Hi Alice!",
  "used_template": "Hi {first_name}!",
  "customer": {
    "id": 1,
    "first_name": "Alice"
  }
}
```

### 5. Send Campaign to Customers

```bash
curl -X POST http://localhost:8080/api/campaigns/1/send \
  -H "Content-Type: application/json" \
  -d '{
    "customer_ids": [1, 2, 3, 4, 5]
  }'
```

**Expected Response:**

```json
{
  "campaign_id": 1,
  "messages_queued": 5,
  "status": "sending"
}
```

**Test Idempotency:**

```bash
# Try sending again - should get 409 Conflict
curl -X POST http://localhost:8080/api/campaigns/1/send \
  -H "Content-Type: application/json" \
  -d '{
    "customer_ids": [1, 2, 3, 4, 5]
  }'
```

**Expected Response (409 Conflict):**

```json
{
  "error": {
    "code": "CONFLICT",
    "message": "campaign already processed (status: 'sending'). To prevent duplicate sends, campaigns in 'sending', 'sent', or 'failed' status cannot be sent again"
  }
}
```

### 6. Monitor Worker Logs

```bash
# Follow worker logs in real-time
docker-compose logs -f worker

# Check recent processing activity
docker-compose logs --tail=50 worker | grep "processing message"

# Check for failures
docker-compose logs worker | grep "message send failed"
```

### 7. Check Campaign Completion

```bash
# Wait a few seconds for messages to process, then check stats
curl http://localhost:8080/api/campaigns/1
```

**Expected Response (after processing):**

```json
{
  "id": 1,
  "status": "sent",
  "stats": {
    "total": 5,
    "pending": 0,
    "sending": 0,
    "sent": 4,
    "failed": 1
  }
}
```

## Extra Feature: Idempotency

### What Was Implemented

**Application-level idempotency** to prevent duplicate campaign sends if the API is called multiple times.

### How It Works

The system uses campaign status as a state machine to enforce idempotency:

```txt
POST /api/campaigns/1/send (First Call)
  ↓
Status: "draft" → Process messages → Status: "sending"
✓ Returns: {"campaign_id": 1, "messages_queued": 5, "status": "sending"}

POST /api/campaigns/1/send (Second Call)
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
- **Manual trigger required**: Call `/api/campaigns/{id}/send` when ready
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

**Total Time**: Approximately **8-12 hours**

**Breakdown**:

- Initial setup and architecture design: ~1 hour
- Database schema and migrations: ~1 hour
- Repository and service layer implementation: ~2-3 hours
- HTTP handlers and API endpoints: ~2-3 hours
- Queue integration and worker implementation: ~3 hours
- Testing, debugging, and spec compliance fixes: ~2 hours
- Documentation (README, SYSTEM_OVERVIEW, code comments): ~1 hour

**AI Tools Used**:

This project was developed **with assistance from AI tools**, specifically:

1. **Claude Code** (Anthropic):

   - Architecture guidance and design decisions
   - Implementation of repository pattern and clean architecture
   - Error handling best practices
   - Code review and bug fixes
   - Documentation writing and structuring

2. **GitHub Copilot**:

   - Boilerplate code generation (struct definitions, SQL queries)
   - Auto-completion for repetitive patterns
   - Test case generation

3. **ChatGPT** (OpenAI):
   - Research on Go production best practices (2025 standards)
   - Queue system comparisons (Redis vs RabbitMQ)
   - Idempotency patterns and strategies

**Human Contributions**:

- Overall system design and architectural decisions
- Business logic implementation
- Database schema design and indexing strategy
- Manual testing and verification
- Debugging and problem-solving
- Final code review and refinements

## Frontend

**Status**: Not implemented

This project focuses exclusively on the **backend implementation**. No frontend was built.

## License

This is a take-home coding challenge. Code may be used for evaluation purposes.

## Author

## Raymond Madara
