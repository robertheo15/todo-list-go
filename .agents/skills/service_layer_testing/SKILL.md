---
name: service_layer_testing
description: Guidelines, patterns, and architectural standards for writing comprehensive, race-safe unit tests covering internal/service in Go 1.25. Covers httptest.Server mocking of external weather APIs, table-driven test patterns, context and zerolog logger injection, concurrent worker (fetchWeatherConcurrently) verification, and race detection.
---

# Service Layer Unit Testing Skill (`service_layer_testing`)

This skill defines the mandatory testing standards and patterns for unit testing business logic and upstream API orchestration inside `internal/service` (`weather_service.go`, `stats_service.go`).

## 1. Core Testing Principles for the Service Layer

1. **Zero External Network Calls**: Unit tests **must never** contact live external HTTP endpoints (`api.openweathermap.org` or `api.weatherapi.com`) or production Redis servers. All network dependencies must be mocked using `net/http/httptest.Server` or repository interfaces/test doubles.
2. **Table-Driven Tests Only**: All test functions inside `internal/service/` must use Go's standard table-driven test pattern (`[]struct{ name string, ... }`) executed via `t.Run(tc.name, func(t *testing.T) { ... })`.
3. **Mandatory Race Verification**: Every unit test must pass clean under Go's data race detector (`go test -v -race ./internal/service/...`). No test is complete until `-race` returns zero warnings.
4. **Context & Logger Injection**: Because `internal/service` methods require request-scoped `zerolog` loggers (`log.Ctx(ctx)`), tests must initialize a valid `zerolog.Logger` attached to `context.Context` to prevent nil-pointer panics or lost structured logs during execution.

---

## 2. Mocking External HTTP Providers (`httptest.Server`)

When testing `GetOpenWeatherAPI` or concurrent upstream polling (`fetchWeatherConcurrently`), use `httptest.NewServer` to stand up local mock HTTP endpoints and redirect the service's target URL environment variable (`OPENWEATHER_API`) to the test server's `URL`.

### Standard External API Mocking Pattern
```go
func TestGetOpenWeatherAPI(t *testing.T) {
    // 1. Setup Table Cases
    testCases := []struct {
        name        string
        city        string
        handler     http.HandlerFunc
        expectedErr error
    }{
        {
            name: "200 OK - Valid Weather Payload",
            city: "jakarta",
            handler: func(w http.ResponseWriter, r *http.Request) {
                // Verify request parameters passed by service
                if r.URL.Query().Get("q") != "jakarta" {
                    t.Errorf("expected query q=jakarta, got %s", r.URL.Query().Get("q"))
                }
                w.WriteHeader(http.StatusOK)
                w.Write([]byte(`{"name":"Jakarta","main":{"temp":30.5},"weather":[{"main":"Sunny"}]}`))
            },
            expectedErr: nil,
        },
        {
            name: "404 Not Found - Invalid City Code",
            city: "invalid_city",
            handler: func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusNotFound)
                w.Write([]byte(`{"cod":"404","message":"city not found"}`))
            },
            expectedErr: service.ErrInvalidCity,
        },
        {
            name: "500 Internal Server Error - Upstream Down",
            city: "london",
            handler: func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusInternalServerError)
                w.Write([]byte(`Internal Server Error`))
            },
            expectedErr: service.ErrAPIDown,
        },
    }

    for _, tc := range testCases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            // 2. Stand up httptest Server
            mockServer := httptest.NewServer(tc.handler)
            defer mockServer.Close()

            // 3. Override Environment Variables dynamically
            t.Setenv("OPENWEATHER_API", mockServer.URL)
            t.Setenv("OPENWEATHERMAP_KEY", "test-mock-api-key")

            // 4. Create Context enriched with zerolog
            ctx := setupTestContext(t)

            // 5. Instantiate Service & Execute
            svc := service.NewService(nil) // Repository passed as nil if purely testing HTTP fetcher
            result, err := svc.GetOpenWeatherAPI(ctx, tc.city)

            // 6. Assert Expectations
            if tc.expectedErr != nil {
                if !errors.Is(err, tc.expectedErr) {
                    t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if result.City != "Jakarta" {
                t.Errorf("expected city Jakarta, got %s", result.City)
            }
        })
    }
}
```

---

## 3. Testing Concurrent Fan-Out & Result Collection (`collectWeatherResults`)

When testing the concurrency orchestration methods (`fetchWeatherConcurrently` and `collectWeatherResults`), verify:
1. **First-Valid-Wins Behavior**: If one API returns an error or takes 2 seconds and another API returns valid data in 10ms, `collectWeatherResults` immediately returns the valid response.
2. **Channel Polling & Loop Exit**: Verify that when all workers fail (`totalWorkers` errors received), `collectWeatherResults` does not hang and correctly aggregates and returns the first domain error (`ErrInvalidCity` or `ErrAPIDown`).

### Testing `collectWeatherResults` Synchronization
```go
func TestCollectWeatherResults(t *testing.T) {
    ctx := setupTestContext(t)
    svc := service.NewService(nil)

    t.Run("Returns first valid result and ignores errors", func(t *testing.T) {
        resultCh := make(chan *models.WeatherModel, 2)
        errorCh := make(chan error, 2)

        // Simulate 2 workers: 1 error, 1 success
        errorCh <- service.ErrAPIDown
        resultCh <- &models.WeatherModel{City: "Tokyo", Source: "MockProvider"}
        close(resultCh)
        close(errorCh)

        res, err := svc.CollectWeatherResultsForTest(ctx, 2, resultCh, errorCh)
        if err != nil {
            t.Fatalf("expected success, got error: %v", err)
        }
        if res.City != "Tokyo" {
            t.Errorf("expected Tokyo, got %s", res.City)
        }
    })

    t.Run("Returns ErrInvalidCity when all workers fail", func(t *testing.T) {
        resultCh := make(chan *models.WeatherModel, 2)
        errorCh := make(chan error, 2)

        errorCh <- errors.New("timeout")
        errorCh <- service.ErrInvalidCity
        close(resultCh)
        close(errorCh)

        _, err := svc.CollectWeatherResultsForTest(ctx, 2, resultCh, errorCh)
        if !errors.Is(err, service.ErrInvalidCity) {
            t.Fatalf("expected ErrInvalidCity, got %v", err)
        }
    })
}
```

---

## 4. Context & Logger Setup Helper (`setupTestContext`)

In all test files inside `internal/service/`, include a standardized helper to inject `zerolog` into `context.Context`:
```go
func setupTestContext(t *testing.T) context.Context {
    t.Helper()
    // Create an in-memory test logger writing directly to t.Log via zerolog.TestWriter
    testLogger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
    return testLogger.WithContext(context.Background())
}
```

---

## 5. Repository Layer Abstraction & Caching Tests

To unit test `FetchWeatherFromAPIs` (`getWeatherFromCache` + `cacheWeatherResult`), you can:
1. **Use `httptest.Server` with `redis-mock` / `miniredis`**: If keeping the concrete pointer `*repository.RedisRepository`, spin up `miniredis` in the test setup and pass `repository.NewRedisCacheRepository(miniredisClient)` into `NewService(...)`.
2. **Interface Abstraction (`service.WeatherRepository`)**: If isolating purely from Redis implementation details, define an interface in `service.go`:
   ```go
   type WeatherRepository interface {
       GetCachedRepository(ctx context.Context, key string, response *models.WeatherModel) error
       SetCachedRepository(ctx context.Context, response *models.WeatherModel) error
   }
   ```
   This allows injecting mock struct doubles (`type MockRepo struct { ... }`) directly into `Service` during unit tests.

---

## 6. Execution Command Checklist

Run tests specifically for `internal/service` with race detection enabled:
```bash
# Run unit tests across the service layer with verbose output & race detector
go test -v -race ./internal/service/...

# Run with coverage report
go test -v -race -coverprofile=coverage.out ./internal/service/...
go tool cover -func=coverage.out
```
