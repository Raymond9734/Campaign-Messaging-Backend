# CampaignManager  - System Overview

## Data Model and Entity Relationships

### Entity Relationship Diagram

```txt
┌─────────────────┐
│   customers     │
├─────────────────┤
│ id (PK)         │◀──┐
│ phone           │   │
│ first_name      │   │
│ last_name       │   │
│ location        │   │
│ preferred       │   │
│  _product       │   │
└─────────────────┘   │
                      │
                      │
┌─────────────────┐   │    ┌──────────────────┐
│   campaigns     │   │    │ outbound_messages│
├─────────────────┤   │    ├──────────────────┤
│ id (PK)         │◀──┼────│ id (PK)          │
│ name            │   │    │ campaign_id (FK) │
│ channel         │   └────│ customer_id (FK) │
│ status          │        │ status           │
│ base_template   │        │ rendered_content │
│ scheduled_at    │        │ last_error       │
│ created_at      │        │ retry_count      │
└─────────────────┘        │ created_at       │
                           │ updated_at       │
                           └──────────────────┘
```

### Table Descriptions

#### **customers**

Stores customer information for campaign targeting.

**Columns:**

- `id` - Primary key
- `phone` - Customer phone number (required)
- `first_name`, `last_name` - Customer name
- `location` - Customer location
- `preferred_product` - Product preference for personalization

**Key Indexes:**

- `idx_customers_phone` - Fast phone number lookups

**Business Rules:**

- Phone is required (used for message delivery)
- Other fields optional (empty strings for missing data)

#### **campaigns**

Campaign definitions with message templates.

**Columns:**

- `id` - Primary key
- `name` - Campaign name
- `channel` - sms or whatsapp
- `status` - draft/scheduled/sending/sent/failed
- `base_template` - Message template with {placeholders}
- `scheduled_at` - Optional scheduled send time
- `created_at` - Campaign creation timestamp

**Status Values:**

- `draft` - Created, not sent yet
- `scheduled` - Has scheduled_at timestamp
- `sending` - Currently being processed (worker updates to 'sent' when done)
- `sent` - All messages processed successfully
- `failed` - All messages failed

**Channel Values:**

- `sms` - SMS delivery
- `whatsapp` - WhatsApp delivery

**Key Indexes:**

- `idx_campaigns_status` - Filter by status
- `idx_campaigns_channel` - Filter by channel
- `idx_campaigns_id_desc` - Stable pagination (newest first)
- `idx_campaigns_scheduled` - Find campaigns ready to send

**Business Rules:**

- base_template must contain valid placeholders
- scheduled_at is optional (NULL = send immediately)

#### **outbound_messages**

Individual messages to be sent to customers.

**Columns:**

- `id` - Primary key
- `campaign_id` - Foreign key to campaigns
- `customer_id` - Foreign key to customers
- `status` - pending/sent/failed (no "sending" status)
- `rendered_content` - Pre-rendered message with customer data
- `last_error` - Error message for failed sends
- `retry_count` - Number of send attempts (max 3)
- `created_at`, `updated_at` - Timestamps

**Status Transitions:**

```md
pending → sent (success)
pending → failed (failure after retries)
```

**Key Indexes:**

- `idx_outbound_messages_campaign_status` - Campaign statistics (GROUP BY)
- `idx_outbound_messages_worker_queue` - Worker fetches pending messages
- `idx_outbound_messages_retry` - Retry logic queries

**Business Rules:**

- rendered_content is pre-computed (template + customer data)
- retry_count increments on each failure
- Max retries: 3 (configurable)
- last_error stores failure reason
- Messages have no "sending" status (only pending/sent/failed)

### Foreign Key Relationships

#### outbound_messages.campaign_id → campaigns.id\*\*

- `ON DELETE CASCADE` - Deleting campaign removes all messages

#### outbound_messages.customer_id → customers.id\*\*

- `ON DELETE CASCADE` - Deleting customer removes their messages

### Indexes Rationale

1. **Pagination Stability**: `ORDER BY id DESC` prevents duplicates/missing records
2. **Stats Performance**: Composite index `(campaign_id, status)` enables fast GROUP BY
3. **Worker Efficiency**: Index on `(status, created_at)` for `WHERE status='pending'` queries
4. **Filter Performance**: Separate indexes on commonly filtered fields

### Stats Response Note

The API returns campaign statistics in this format:

```json
{
  "stats": {
    "total": 100,
    "pending": 45,
    "sending": 0,
    "sent": 50,
    "failed": 5
  }
}
```

**Important**: The `"sending"` field is **always 0** in our implementation because outbound messages only have three statuses: `pending`, `sent`, and `failed`. There is no in-flight "sending" status for individual messages.

---

## Request Flow: POST /campaigns/{id}/send

### High-Level Flow

```txt
┌─────────┐     ┌─────────┐     ┌──────────┐     ┌────────┐
│ Client  │────▶│   API   │────▶│  Redis   │────▶│ Worker │
└─────────┘     └────┬────┘     └──────────┘     └───┬────┘
                     │                                │
                     └────────────┬───────────────────┘
                                  │
                             ┌────▼─────┐
                             │PostgreSQL│
                             └──────────┘
```

### Detailed Request Flow

#### Phase 1: API Handler (HTTP Layer)

```md
1. Client → POST /campaigns/1/send
   Body: {"customer_ids": [1, 2, 3]}

2. CampaignHandler.SendCampaign()
   - Extract campaign_id from URL path
   - Parse JSON request body
   - Validate request structure
```

#### Phase 2: Service Layer (Business Logic)

```txt
3. CampaignService.SendCampaign(ctx, campaignID, request)

   a. Fetch Campaign from DB
      - Check if exists (404 if not)
      - Validate status (must be "draft" or "scheduled")
      - Return 409 Conflict if already "sending"/"sent"

   b. For each customer_id:
      i.   Fetch Customer from DB
      ii.  Render template with customer data
           Template: "Hi {first_name}, check {preferred_product}!"
           Customer: {first_name: "Alice", preferred_product: "Shoes"}
           Result:   "Hi Alice, check Shoes!"
      iii. Create OutboundMessage struct:
           {
             campaign_id: 1,
             customer_id: 1,
             status: "pending",
             rendered_content: "Hi Alice, check Shoes!",
             retry_count: 0
           }

   c. Batch Insert Messages to DB
      - Single transaction (performance)
      - All messages created atomically

   d. Publish Jobs to Redis Queue
      - For each message:
        LPUSH campaign_sends {"outbound_message_id": 123}
      - Jobs queued for worker processing

   e. Update Campaign Status
      - SET status = 'sending'
      - Indicates processing started
      - Worker will update to 'sent' when all messages complete
```

#### Phase 3: Response

```txt
4. Return Success Response
   {
     "campaign_id": 1,
     "messages_queued": 3,
     "status": "sending"
   }
```

### Error Handling

#### 404 Not Found

- Campaign doesn't exist
- Customer doesn't exist

#### 400 Bad Request

- Invalid JSON
- Missing customer_ids
- Empty customer_ids array

#### 409 Conflict

- Campaign already sent
- Campaign status not "draft"/"scheduled"

#### 500 Internal Server Error

- Database connection failure
- Redis connection failure
- Template rendering error (logged, not exposed)

---

## Queue Worker Processing Logic

### Worker Lifecycle

```txt
Start Worker
   │
   ├─→ Connect to PostgreSQL
   ├─→ Connect to Redis
   ├─→ Initialize MessageProcessor
   │
   └─→ Start Consuming Loop
        │
        └─→ [Blocking BRPOP from Redis]
             │
             ├─→ Job received
             │   │
             │   ├─→ Process(job)
             │   │
             │   └─→ Continue loop
             │
             └─→ Context canceled (SIGTERM)
                 │
                 └─→ Graceful shutdown (wait 5s)
```

### Message Processing Flow

```txt
1. Worker calls: queueClient.Consume(ctx, handler)

2. Redis BRPOP (blocking, timeout 1s)
   - Waits for job in "campaign_sends" queue
   - Returns: {"outbound_message_id": 123}

3. Handler(ctx, job) triggered
   │
   └─→ MessageProcessor.Process(ctx, job)
       │
       ├─→ a. Fetch outbound_message from DB (by ID)
       │
       ├─→ b. Fetch campaign (for channel info)
       │
       ├─→ c. Fetch customer (for phone number)
       │
       ├─→ d. Call MockSender.Send(channel, phone, content)
       │    - Simulates 50-200ms network latency
       │    - 92% success rate, 8% random failure
       │
       ├─→ e. Handle Success
       │   - UPDATE outbound_messages SET status='sent'
       │   - Check if all campaign messages complete
       │   - If all complete: UPDATE campaigns SET status='sent'
       │   - Log success
       │
       └─→ f. Handle Failure
           - INCREMENT retry_count
           - IF retry_count >= 3:
               UPDATE status='failed', last_error='max retries exceeded'
               Check if all campaign messages complete
               If all complete: UPDATE campaigns SET status='sent' or 'failed'
           - ELSE:
               UPDATE status='failed', last_error='network error'
               (can be manually re-queued)
           - Log failure with retry info

4. Loop back to step 2
```

### Retry Logic Detail

```txt
Attempt 1: Send fails
├─→ retry_count: 0 → 1
├─→ status: 'pending' → 'failed'
├─→ last_error: 'simulated network error'
└─→ Job consumed from queue (not auto-requeued)

Attempt 2: (If manually re-queued)
├─→ retry_count: 1 → 2
├─→ status: 'failed' (remains)
└─→ last_error: updated

Attempt 3: (Max retries)
├─→ retry_count: 2 → 3
├─→ status: 'failed' (permanent)
├─→ last_error: 'max retries exceeded: ...'
└─→ No further retries
```

**Note**: Current implementation does NOT auto-requeue. In production, implement:

- Exponential backoff (1s, 2s, 4s delays)
- Dead letter queue (DLQ) for permanently failed messages
- Monitoring and alerting on failure rates

### Campaign Status Update Logic

After processing each message (whether success or permanent failure), the worker checks if all messages for the campaign are complete:

```go
func updateCampaignStatusIfComplete(ctx, campaignID) {
    // Get campaign with stats
    campaign := getCampaignWithStats(campaignID)

    // Check if all messages complete (no pending messages)
    if campaign.Stats.Pending > 0 {
        return  // Still processing
    }

    // Determine final status
    if campaign.Stats.Failed > 0 && campaign.Stats.Sent == 0 {
        newStatus = "failed"    // ALL messages failed
    } else {
        newStatus = "sent"      // At least SOME messages sent successfully
    }

    // Update campaign status
    updateCampaignStatus(campaignID, newStatus)
}
```

**Status Transition Rules:**

- Campaign starts as `"sending"` when send is initiated
- Worker updates to `"sent"` if any messages succeeded (partial success counts as sent)
- Worker updates to `"failed"` only if ALL messages failed
- Status update happens after the last message completes (pending = 0)

### Concurrency Model

```txt
Worker Process
  │
  ├─→ Main Goroutine: queueClient.Consume()
  │   └─→ Fetches jobs from Redis queue
  │
  ├─→ Worker Pool: Up to N concurrent goroutines (default: 5)
  │   ├─→ Goroutine 1: Processing Message #1
  │   ├─→ Goroutine 2: Processing Message #2
  │   ├─→ Goroutine 3: Processing Message #3
  │   ├─→ Goroutine 4: Processing Message #4
  │   └─→ Goroutine 5: Processing Message #5
  │
  └─→ Signal Handler Goroutine
      └─→ Listens for SIGTERM/SIGINT
```

**Concurrency Control:**

- **Semaphore-based**: Limits concurrent message processing
- **Configured via `WORKER_CONCURRENCY`** env var (default: 5, max: 5)
- **Graceful shutdown**: Waits for in-flight jobs to complete
- **Resource-efficient**: Prevents database connection pool exhaustion

**How It Works:**

1. Main loop continuously fetches jobs from Redis (BRPOP)
2. For each job, acquires a semaphore slot (blocks if all 5 slots busy)
3. Spawns goroutine to process the job
4. Goroutine releases semaphore slot when done
5. Maximum 5 messages processed concurrently at any time

**For Even Higher Throughput:**

- Run multiple worker instances (horizontal scaling)
- Each instance can process up to 5 messages concurrently
- Example: 3 worker instances × 5 concurrency = 15 messages processed simultaneously

---

## Pagination Strategy

### Requirements

- No duplicate records between pages
- No missing records
- Consistent ordering across pages
- Works while new data is being inserted

### Implementation

**Query Pattern:**

```sql
SELECT * FROM campaigns
WHERE 1=1
  AND channel = $1  -- optional filter
  AND status = $2   -- optional filter
ORDER BY id DESC    -- stable, consistent
LIMIT $3 OFFSET $4
```

**Why `ORDER BY id DESC`?**

1. **Stability**: ID is immutable (never changes)
2. **Consistency**: Same order every time
3. **Performance**: Index on `id` (primary key, always indexed)
4. **No Duplicates**: Even if new campaigns created during paging

### Comparison with Alternatives

#### ORDER BY created_at

- Problem: Multiple records can have same timestamp
- Result: Non-deterministic ordering, possible duplicates

#### Cursor-based pagination

- Better for large datasets
- More complex implementation
- Requires encoding cursor state
- Overkill for this use case

#### ORDER BY id DESC + OFFSET

- Simple, predictable
- Good performance with indexes
- Standard pattern
- Adequate for expected dataset size

### Pagination Validation

```go
// Defaults and limits
if page < 1 {
    page = 1
}
if pageSize < 1 {
    pageSize = 20  // default
}
if pageSize > 100 {
    pageSize = 100  // maximum
}

offset := (page - 1) * pageSize
```

### Response Format

```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total_count": 57,
    "total_pages": 3
  }
}
```

**total_pages calculation:**

```md
total_pages = ceil(total_count / page_size)
```

---

## Personalization Approach

### Current Implementation: Template Substitution

**Pattern:** Simple regex-based placeholder replacement

```md
Template: "Hi {first_name}, {preferred_product} is on sale!"
Customer: {first_name: "Bob", preferred_product: "Laptop"}
Result: "Hi Bob, Laptop is on sale!"
```

**Implementation:**

```go
placeholderPattern := regexp.MustCompile(`\{([a-z_]+)\}`)

result := placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
    fieldName := strings.Trim(match, "{}")
    return fieldMap[fieldName]  // or "" if missing
})
```

### Valid Placeholders

| Placeholder           | Source                      | Example         |
| --------------------- | --------------------------- | --------------- |
| `{first_name}`        | customers.first_name        | "Alice"         |
| `{last_name}`         | customers.last_name         | "Mwangi"        |
| `{location}`          | customers.location          | "Nairobi"       |
| `{preferred_product}` | customers.preferred_product | "Running Shoes" |
| `{phone}`             | customers.phone             | "+254712345001" |

### Missing Field Behavior

**Design Decision**: Replace with empty string (not error)/ (Use generic words e.g Customer)

**Rationale:**

- Campaigns can proceed with incomplete data
- Better UX than blocking on missing fields
- Allows gradual customer data enrichment

**Example:**

```md
Template: "Hi {first_name} {last_name}"
Customer: {first_name: "Alice", last_name: NULL}
Result: "Hi Alice "
```

### Future Extension Points

The current design allows easy enhancement with AI-powered personalization:

#### 1. **Dynamic Content Generation**

```go
type AIPersonalizer interface {
    Personalize(ctx context.Context, customer *Customer, context string) (string, error)
}

// In CampaignService.SendCampaign():
if campaign.UseAI {
    content, _ = aiPersonalizer.Personalize(ctx, customer, campaign.Context)
} else {
    content, _ = templateSvc.Render(campaign.BaseTemplate, customer)
}
```

#### 2. **Multi-Language Support**

```go
type LocalizationService interface {
    Translate(text, targetLanguage string) (string, error)
}

// Detect customer language from location/profile
language := detectLanguage(customer)
translated := localizationSvc.Translate(content, language)
```

#### 3. **A/B Testing**

```go
// Randomly assign template variants
variant := selectVariant(campaign.Variants, customer.ID)
content, _ = templateSvc.Render(variant.Template, customer)
```

#### 4. **Real-time Product Recommendations**

```go
type RecommendationEngine interface {
    GetRecommendations(customerID int64) ([]Product, error)
}

// Inject recommendations into template context
recommendations := recommendationEngine.GetRecommendations(customer.ID)
enrichedData := mergeCustomerWithRecommendations(customer, recommendations)
content, _ = templateSvc.Render(template, enrichedData)
```

### Template Validation

Templates are validated at creation time:

```go
func (s *templateService) ValidateTemplate(template string) error {
    placeholders := s.ExtractPlaceholders(template)

    validPlaceholders := map[string]bool{
        "first_name": true,
        "last_name": true,
        "location": true,
        "preferred_product": true,
        "phone": true,
    }

    for _, placeholder := range placeholders {
        if !validPlaceholders[placeholder] {
            return ErrInvalidPlaceholder
        }
    }

    return nil
}
```

**Benefits:**

- Catch errors early (at campaign creation)
- Prevent runtime failures
- Clear error messages to API users

---

## Conclusion

The design prioritizes simplicity, correctness, and extensibility over premature optimization.
