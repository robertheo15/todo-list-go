---
name: concurrent_api_fetcher
description: Patterns and strict guardrails for implementing concurrent multi-provider HTTP polling, fan-out/fan-in goroutine workers, channel synchronization, and timeout management in Go.
---

# Concurrent API Fetcher Skill (`concurrent_api_fetcher`)

This skill governs the concurrent fan-out and result-collection patterns used when polling multiple external weather services (`OpenWeatherMap`, `WeatherAPI`) simultaneously within `internal/service/weather_service.go`.

## 1. Concurrent Polling Architecture (`Fan-Out / Fan-In`)

When `FetchWeatherFromAPIs` encounters a Redis cache miss, it spawns goroutines to query all active weather providers concurrently (`fetchWeatherConcurrently`). Whichever provider returns a valid response first (`collectWeatherResults`) wins, cancelling the remaining request timeouts and returning to the client with minimum latency.

```
                  ┌─► Goroutine 1 (OpenWeatherMap) ─┐
                  │                                  ▼
FetchWeather ────┼─► Goroutine 2 (WeatherAPI)     ───► collectWeatherResults (select loop)
 (Cache Miss)     │                                  ▲
                  └─► Goroutine N (Future API)     ──┘
```

---

## 2. The Channel Polling Flaw (`CRITICAL TRAP`)

When polling unbuffered or buffered channels using a `select` loop inside `collectWeatherResults`, you must never check `len(resultCh) == 0 && len(errorCh) == 0` to decide when to break!

### Why `len() == 0` Is Broken
Checking `len(ch) == 0` only tells you whether items are *currently waiting in the channel buffer at that exact microsecond*, not whether all goroutines have finished running. If an early provider fails fast (`errorCh` popped) while a slower valid provider is still computing, the loop sees `len(resultCh) == 0 && len(errorCh) == 0` as `true` and exits with an error immediately!

---

## 3. Correct Fan-Out & Synchronization Implementation

Use explicit worker counting (`receivedCount < totalWorkers`) and channel closure checks (`res, ok := <-resultCh`) to guarantee race-free concurrency:

### Step 1: Fan-Out (`fetchWeatherConcurrently`)
```go
func (s *Service) fetchWeatherConcurrently(ctx context.Context, city string) (*models.WeatherModel, error) {
    logger := log.Ctx(ctx).With().Str("func", "fetchWeatherConcurrently").Logger()
    logger.Debug().Str("city", city).Msg("spawning concurrent API workers")

    var wg sync.WaitGroup

    // Enforce hard 10-second ceiling across all concurrent requests
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    apis := []func(context.Context, string) (*models.WeatherModel, error){
        s.GetOpenWeatherAPI,
        // s.GetWeatherAPI,
    }

    resultCh := make(chan *models.WeatherModel, len(apis))
    errorCh := make(chan error, len(apis))

    for _, apiFunc := range apis {
        wg.Add(1)
        go func(worker func(context.Context, string) (*models.WeatherModel, error)) {
            defer wg.Done()
            if data, err := worker(ctx, city); err != nil {
                errorCh <- err
            } else {
                resultCh <- data
            }
        }(apiFunc)
    }

    // Close channels when all goroutines exit
    go func() {
        wg.Wait()
        close(resultCh)
        close(errorCh)
    }()

    return s.collectWeatherResults(ctx, len(apis), resultCh, errorCh)
}
```

### Step 2: Result Collection (`collectWeatherResults`)
```go
func (s *Service) collectWeatherResults(
    ctx context.Context,
    totalWorkers int,
    resultCh <-chan *models.WeatherModel,
    errorCh <-chan error,
) (*models.WeatherModel, error) {
    logger := log.Ctx(ctx).With().Str("func", "collectWeatherResults").Logger()
    var errorsList []error
    receivedCount := 0

    for receivedCount < totalWorkers {
        select {
        case res, ok := <-resultCh:
            if ok && res != nil {
                logger.Debug().Str("source", res.Source).Msg("received valid weather result from worker")
                return res, nil // First valid response wins!
            }
        case apiErr, ok := <-errorCh:
            if ok && apiErr != nil {
                logger.Warn().Err(apiErr).Msg("API worker returned an error")
                errorsList = append(errorsList, apiErr)
            }
        case <-ctx.Done():
            logger.Error().Msg("timeout exceeded while fetching weather data concurrently")
            return nil, errors.New("timeout exceeded while fetching weather data")
        }
        receivedCount++
    }

    // Inspect collected errors if no valid response arrived
    if len(errorsList) > 0 {
        for _, err := range errorsList {
            if errors.Is(err, ErrInvalidCity) {
                return nil, ErrInvalidCity
            }
        }
        return nil, errorsList[0]
    }

    return nil, errors.New("no weather service responded")
}
```

---

## 4. HTTP Request Timeout Options inside Workers

When creating `http.Client` inside provider functions (`GetOpenWeatherAPI`), always bound the request with `http.NewRequestWithContext(ctx, ...)` so canceling the parent context immediately aborts pending TCP sockets:

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
if err != nil {
    return nil, fmt.Errorf("failed to create request: %w", err)
}

client := &http.Client{Timeout: 10 * time.Second}
response, err := client.Do(req)
if err != nil {
    return nil, fmt.Errorf("%w: %v", ErrAPIDown, err)
}
defer response.Body.Close()
```
