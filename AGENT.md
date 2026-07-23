# AI Agent & Developer Workflow Instructions (`agent.md`)

This document is the authoritative **system context, coding standard, and workflow guide** for any AI coding assistant (e.g., Antigravity, Cursor, Copilot, Cline, Aider) or human developer working on the **Weather API** (`personal-blog-api`) repository.

When prompted to analyze, refactor, debug, or extend this codebase, **you must strictly adhere to the rules, architectural boundaries, and coding patterns outlined in this guide.**

---

## 1. Quick Repository Orientation

- **Language & Version**: Go `1.25` (enforce Go 1.25+ idioms, strong typing, and standard library best practices).
- **Primary Domain**: High-performance HTTP proxy & caching service for external weather providers ([OpenWeatherMap](https://openweathermap.org/) and [WeatherAPI](https://www.weatherapi.com/)).
- **Core Frameworks**:
  - **Router**: [`github.com/go-chi/chi/v5`](personal-blog-api/internal/http/http_server.go) (`v5.2.0`)
  - **Rate Limiter**: [`github.com/go-chi/httprate`](personal-blog-api/internal/http/http_server.go#L53) (IP-based, 60 req/min)
  - **Cache Client**: [`github.com/go-redis/redis/v8`](personal-blog-api/internal/repository/redis_repository.go) (`v8.11.5`)
  - **Logger**: [`github.com/rs/zerolog`](personal-blog-api/internal/config/logger.go) (`v1.33.0`)
  - **UUID / Tracing**: [`github.com/google/uuid`](personal-blog-api/internal/http/correlation_middleware.go) (`v1.6.0`)
  - **Env Loader**: [`github.com/joho/godotenv`](personal-blog-api/internal/config/envFile.go) (`v1.5.1`)

### Directory & Package Layer Boundaries
```
personal-blog-api/
├── cmd/main.go               # Entry point ONLY. Handles wiring, server run, and graceful shutdown.
├── internal/config/          # Global configs, zerolog setup (logger.go), Redis pool (redis.go), and .env loader.
├── internal/http/            # Chi router, handlers (weather_handler.go, stats_handler.go), and correlation middleware.
├── internal/models/          # Domain structs, DTOs, external API response schemas, and unified WeatherModel.
├── repository/     # Data access layer (Redis Cache Get/Set operations).
├── service/        # Business logic, concurrent upstream polling, and cache-first orchestration.
└── pkg/http/                 # Shared utilities, standardized JSON response formatters (response.go).
```

> [!IMPORTANT]
> **Strict Layer Separation**: Never violate Clean Architecture dependency rules:
> `cmd` ➡️ `internal/http` ➡️ `internal/service` ➡️ `internal/repository`
> - `internal/http` must **never** directly access `internal/repository` or external APIs.
> - `internal/service` must **never** import `net/http` handlers or write `http.ResponseWriter` outputs.
> - `internal/models` must remain pure domain definitions with no external side-effects.

---

## 2. Mandatory Coding Rules & Architectural Standards

### 2.1. Structured & Correlation Logging (`zerolog`)
All logging in this codebase uses `zerolog` with request-scoped tracing. **Never use `fmt.Println`, `fmt.Printf`, `log.Println`, or standard `log` in production code.**

#### Rule 1: Always Extract Logger from Context in Request Workflows
In any HTTP handler, service method, or repository call that receives `ctx context.Context`, retrieve the request-scoped logger and inject the current function name:
```go
func (s *Service) DoBusinessLogic(ctx context.Context, param string) error {
    // 1. Extract logger from context and append function name
    logger := log.Ctx(ctx).With().Str("func", "DoBusinessLogic").Logger()

    logger.Debug().Str("param", param).Msg("starting business logic execution")

    if err := someOperation(); err != nil {
        logger.Error().Err(err).Str("param", param).Msg("operation failed")
        return fmt.Errorf("operation failed: %w", err)
    }
    return nil
}
```

#### Rule 2: Preserve Correlation IDs (`X-Request-ID`)
The [`CorrelationLoggerMiddleware`](personal-blog-api/internal/http/correlation_middleware.go#L10) automatically assigns an `X-Request-ID` and attaches `reqLogger.WithContext(r.Context())`. When spawning child goroutines during concurrent fetching, you **must** pass `ctx` (or a derived timeout context) into the goroutine so `log.Ctx(ctx)` continues to log the exact `request_id`.

---

### 2.2. Redis Caching Protocol (`internal/repository`)
When reading from or writing to Redis via [`RedisRepository`](personal-blog-api/internal/repository/redis_repository.go):
1. **Key Normalization**: Always normalize keys to lowercase using `strings.ToLower(key)` to avoid cache misses due to casing (`Jakarta` vs `jakarta`).
2. **Standard Expiration (TTL)**: All cached weather records must be stored with a **12-hour TTL** (`12 * time.Hour`), matching the `EX` flag specification.
3. **Error Wrapping (`redis.Nil`)**: When `redis.Get` fails on a cache miss, wrap the error using `%w` so callers can inspect `errors.Is(err, redis.Nil)`:
   ```go
   // CORRECT:
   if err != nil {
       return fmt.Errorf("redis.Get key=%s: %w", key, err)
   }
   // INCORRECT (breaks errors.Is checks):
   // return errors.New(fmt.Sprintf("redis.Get key=%s err = %s", key, err.Error()))
   ```

---

### 2.3. Standardized HTTP Responses & Headers (`pkg/http`)
All HTTP handler responses must use the standardized JSON envelope utilities defined in [`pkg/http/response.go`](personal-blog-api/pkg/http/response.go):
- Success: `pkgHttp.SetResponse(w, http.StatusOK, data, "message", true)`
- Error: `pkgHttp.SetError(w, httpStatus, err)`

> [!WARNING]
> **Check `Content-Type` Headers**: When modifying or refactoring `pkg/http/response.go`, ensure that `w.Header().Set("Content-Type", "application/json")` is explicitly called **before** `w.WriteHeader(httpStatus)`.

---

### 2.4. Concurrency & Goroutine Safety
The service layer uses concurrent workers (`fetchWeatherConcurrently`) to poll multiple external weather APIs simultaneously. When modifying concurrent code:
1. **Context Timeouts**: Always wrap background or concurrent network operations in `context.WithTimeout(ctx, 10*time.Second)` to prevent leaked goroutines or hanging requests.
2. **Thread-Safe Counter Mutations**: In-memory statistics (`s.apiUsage`, `s.cacheHits`) accessed across concurrent HTTP handlers must be thread-safe. Use `sync/atomic` (`atomic.AddInt64(&s.apiUsage, 1)`) or a `sync.RWMutex`.
3. **Channel Synchronization**: When reading from multi-producer channels (`resultCh`, `errorCh`), track received response counts against total spawned workers or verify channel closure (`res, ok := <-resultCh`) to avoid exiting premature polling loops while workers are still executing.

---
### 2.5. Response data API
The response supposed to be in the following format for all endpoints:
```go
type DataTemplate struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
}

type ErrorMsg struct {
	Error string `json:"error"`
}

func SetResponse(w http.ResponseWriter, httpStatus int, data interface{}, message string, success bool) {
	response := DataTemplate{
		Message: message, Success: success,
		Status: http.StatusText(httpStatus),
		Data:   data,
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		log.Error().Err(err).Msg("[Util][WriteSuccess] marshalling response")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(encoded)
}

func SetError(w http.ResponseWriter, httpStatus int, err error) {
	response := ErrorMsg{
		Error: "Internal Server Error",
	}

	encoded, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		log.Error().Err(marshalErr).Msg("[Util][WriteError] marshalling response")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(encoded)
}

func ReturnInternalServerError(w http.ResponseWriter) {
	response := ErrorMsg{
		Error: "Internal Server Error",
	}

	encoded, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		log.Error().Err(marshalErr).Msg("[Util][WriteError] marshalling response")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(encoded)
}
```
---
## 3. Known Technical Debt & AI Agent Refactoring Guardrails

If you are asked to refactor, debug, or improve existing files, watch out for these exact areas identified during architectural auditing:

| File / Component | Known Issue / Technical Flaw | Required AI Agent Action / Guardrail |
| :--- | :--- | :--- |
| [`redis_repository.go`](personal-blog-api/internal/repository/redis_repository.go#L39) (`GetCachedRepository`) | `errors.New(...)` string wrapping destroys the `redis.Nil` error chain. | Replace with `fmt.Errorf("redis.Get key=%s: %w", key, err)`. |
| [`pkg/http/response.go`](personal-blog-api/pkg/http/response.go#L32) (`SetResponse` / `SetError`) | Writes JSON without `Content-Type: application/json` header. | Add `w.Header().Set("Content-Type", "application/json")` before `WriteHeader`. |

---

## 4. Common Agent Workflow Recipes

### 4.1. Adding a New External Weather Provider
To integrate a new provider (or re-enable `WeatherAPI`):
1. **Model Definition**: Create raw JSON struct schemas in [`internal/models/`](personal-blog-api/internal/models/).
2. **DTO Mapper**: Add a converter function in [`internal/models/weather.go`](personal-blog-api/internal/models/weather.go) returning `*models.WeatherModel`.
3. **Provider Client**: Implement `Get<ProviderName>API(ctx context.Context, city string) (*models.WeatherModel, error)` in [`internal/service/weather_service.go`](personal-blog-api/internal/service/weather_service.go). Ensure `os.Getenv` checks and `log.Ctx(ctx)` tracing are included.


### 4.2. Adding a New API Endpoint
1. **Define Handler**: Add method `func (s *Server) HandleNewRoute(w http.ResponseWriter, r *http.Request)` in `internal/http/`.
2. **Mount Route**: Register in [`internal/http/http_server.go`](personal-blog-api/internal/http/http_server.go#L58) inside `Run()`:
   ```go
   s.router.Get("/new-route", s.HandleNewRoute)
   ```
3. **Service & Repo Integration**: Implement business logic in `internal/service/` and any Redis caching operations in `internal/repository/`.

### 4.3. Writing Unit Tests for the Service Layer (`service_layer_testing`)
When adding or modifying methods in `internal/service/`, consult the authoritative **[Service Layer Testing Skill](personal-blog-api/skills/service_layer_testing/SKILL.md)** (`skills/service_layer_testing/SKILL.md`) and use the template at [`skills/service_layer_testing/examples/service_unit_test.go.template`](personal-blog-api/skills/service_layer_testing/examples/service_unit_test.go.template).
Key rules enforced:
- **No Live External Calls**: Use `httptest.NewServer` to mock upstream weather APIs (`OPENWEATHER_API`) dynamically.
- **Table-Driven Pattern**: Use `[]struct{ name string, ... }` via `t.Run(...)`.
- **Context Logger Injection**: Always enrich test contexts with `testLogger.WithContext(context.Background())` to verify `log.Ctx(ctx)` behavior without panics.

---

## 5. Development & Terminal Command Reference

When running terminal commands on behalf of the user or testing code changes, use these exact commands:

### Dependency & Module Verification
```bash
go mod tidy
go mod vendor -v
```

### Local Application Execution
```bash
# Run server locally (requires running Redis instance on localhost:6379 or configured via .env)
go run cmd/main.go

# Or via Makefile
make build
./personal-blog-api
```

### Docker & Docker Compose Operations
```bash
# Build and start both personal-blog-api and redis containers in background
docker-compose up --build -d

# View live application logs
docker-compose logs -f personal-blog-api

# Stop and tear down containers
docker-compose down
```

### Running Tests & Race Detection
Always verify code changes for race conditions and compilation errors:
```bash
# Run unit tests across all packages with data race detector
go test -v -race ./...

# Run service layer unit tests specifically (with race detector)
go test -v -race ./internal/service/...

# Check formatting and syntax
go vet ./...
```

---

## 6. Checklist for AI Agent Task Completion

Before completing any prompt or marking a task as done, verify:
- [ ] **Clean Architecture**: No layer boundary violations (`cmd` vs `http` vs `service` vs `repository`).
- [ ] **Context Logging**: All touched functions use `log.Ctx(ctx).With().Str("func", "...")` without `fmt.Println`.
- [ ] **Error Handling**: Errors are wrapped (`%w`) with descriptive context.
- [ ] **Cache Consistency**: Cache keys use `strings.ToLower()`, and TTL is preserved at `12 * time.Hour`.
- [ ] **Concurrency & Race Checks**: Concurrent code uses `context.Context`, avoids channel deadlocks/polling bugs, and uses atomic/mutex locking on shared data.
- [ ] **Compilation & Formatting**: Run `go vet ./...` and `go mod tidy` when adding imports or packages.
