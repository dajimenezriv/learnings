# Exercise2 - Go DDD Base

## Setup

```bash
# Install mise
curl -fsSL https://mise.run | sh
echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc
source ~/.bashrc
mise doctor

# Pin dependencies
mise use --pin aqua:go-task/task@3
mise use --pin aqua:golangci/golangci-lint@2.14.0
mise use --pin aqua:sqlc-dev/sqlc@1.31.1
mise use --pin node@24
mise use --pin npm:squawk-cli@2.59.0
mise lock
```

- Terraform github and neon
- Set deleted_by in deletion_audit
- Add grafana cloud and slog
- Neon backups and branches
- Aggregates
- Option types
- Circuit breaker
- Auth middleware
- Prometheus + exemplars
- mcp SERVER
- Redis

---

## 🎯 Live Coding Interview Challenges

This project is designed for conducting 1-hour live coding interviews. Below are proposed challenges ordered by difficulty.

### 1. Add Actor-Account Association ⭐⭐⭐

**Objective:** Create a one-to-many relationship where an Actor can own multiple Accounts

**Sub-tasks:**

- Add `actor_id` foreign key to `accounts` table
- Update Account domain model to include ActorID
- Modify CreateAccount command to require actor_id
- Add validation to ensure Actor exists before creating Account
- Update GET /actors endpoint to include associated accounts
- Handle edge cases (actor doesn't exist, orphaned accounts)

**Skills tested:** Database design, domain modeling, error handling, API design

---

### 2. Add Validation & Business Rules ⭐⭐

**Objective:** Implement proper validation throughout the application

**Sub-tasks:**

- Validate TaxId format (e.g., 9-digit numeric for tax IDs)
- Ensure TaxId uniqueness per Actor
- Add validation for ActorType enum values
- Validate Account keys are not empty
- Return proper HTTP error codes (400 for validation, 409 for conflicts)
- Add unit tests for validation logic

**Skills tested:** Domain logic, error handling, HTTP standards, testing

---

### 3. Add Update Actor Endpoint ⭐⭐

**Objective:** Implement PUT/PATCH `/actors/{id}` to update Actor details

**Sub-tasks:**

- Create UpdateActor command
- Add HTTP handler with proper routing
- Implement update logic in repository
- Handle "not found" scenarios (404)
- Decide: full replacement (PUT) vs partial update (PATCH)
- Test with concurrent updates

**Skills tested:** CQRS pattern, HTTP verbs, repository pattern, concurrency awareness

---

### 4. Add Pagination to GET Endpoints ⭐⭐

**Objective:** Add limit/offset pagination to GET /actors and GET /accounts

**Sub-tasks:**

- Accept query params: `?limit=10&offset=0`
- Update repository queries with LIMIT/OFFSET
- Return metadata: total_count, has_next, has_previous
- Handle invalid pagination params
- Set default and maximum limits

**Skills tested:** Query optimization, API design, SQL knowledge

---

### 5. Add Actor Deletion with Cascade Logic ⭐⭐⭐

**Objective:** Implement DELETE `/actors/{id}` with proper account handling

**Sub-tasks:**

- Decide: cascade delete accounts OR prevent deletion if accounts exist
- Implement soft delete vs hard delete
- Add transaction handling
- Return appropriate status codes
- Update repository with delete method
- Handle referential integrity

**Skills tested:** Database transactions, business decisions, data integrity

---

### 6. Add Filtering & Search ⭐⭐

**Objective:** Add filtering to GET /actors by type and/or taxId

**Sub-tasks:**

- Accept query params: `?type=investor&taxId=123`
- Build dynamic SQL queries safely (avoid SQL injection)
- Update DTO to include filter parameters
- Test with various filter combinations
- Handle no results gracefully

**Skills tested:** SQL composition, security awareness, API design

---

### 7. Add Audit Trail / Created/Updated Timestamps ⭐

**Objective:** Track when entities are created/modified

**Sub-tasks:**

- Add `created_at` and `updated_at` columns to both tables
- Update persistence models and mappers
- Ensure timestamps are set on insert/update
- Return timestamps in API responses
- Consider timezone handling

**Skills tested:** Data modeling, attention to detail, API completeness

---

### 8. Add Health Check Endpoint ⭐

**Objective:** Implement GET /health that checks DB connectivity

**Sub-tasks:**

- Create `/health` endpoint
- Check database connection with `db.Ping()`
- Return JSON: `{"status": "ok", "database": "connected"}`
- Handle degraded states
- Add timestamp to response

**Skills tested:** Production readiness, monitoring, error handling

---

### 9. Add Integration Tests ⭐⭐⭐

**Objective:** Write integration tests for API endpoints

**Sub-tasks:**

- Set up test database connection
- Write tests for POST /actors with success/error cases
- Test GET endpoints with various scenarios
- Use table-driven tests
- Clean up test data properly

**Skills tested:** Testing strategy, Go testing patterns, database setup

---

### 10. Error handling and mapping ⭐⭐⭐

**Objective:** Handle errors gracefully and map them to appropriate HTTP status codes

**Sub-tasks:**

- Create domain error sentinels (or) types
- Map domain errors to HTTP status codes
- Handle errors gracefully
- Specific error responses, in json format, with a proper body and structure
- Test error handling

**Skills tested:** Error handling

---

### 11. Parallel batch creation ⭐⭐⭐⭐

**Objective:** Add a POST /actors/batch endpoint that persists multiple actors concurrently and returns a per-item result

**Sub-tasks:**

- Accept a JSON array of actors and fan out persistence across goroutines
- Synchronize results without data races (sync.WaitGroup + per-index slice or channel)
- Bound concurrency with a worker pool or semaphore
- Return per-item outcomes (created vs. failed) with partial-failure semantics
- (Bonus) Test concurrent creation, including failure cases, with go test -race

**Skills tested:** Goroutines, channels, sync primitives, concurrency control
