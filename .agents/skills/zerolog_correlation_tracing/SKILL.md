---
name: zerolog_correlation_tracing
description: Best practices for structured JSON logging with zerolog, correlation ID propagation (X-Request-ID), context-scoped loggers, and dual-output (os.Stdout + file) setup in the weather-api repository.
---

# Zerolog Correlation & Tracing Skill (`zerolog_correlation_tracing`)

This skill defines mandatory observability patterns using `github.com/rs/zerolog` and `github.com/google/uuid` to ensure complete end-to-end trace correlation across all layers (`internal/http`, `internal/service`, `internal/repository`).

## 1. Core Logging Rules

1. **Structured JSON Only**: All log events must be emitted as structured JSON. Never use `fmt.Println`, `fmt.Printf`, or `log.Printf` inside application code.
2. **Global Debug Level**: By requirement, the global logger must operate at `DebugLevel` (`zerolog.SetGlobalLevel(zerolog.DebugLevel)`).
3. **Context Logger Extraction**: Every function accepting `ctx context.Context` must extract the request-scoped logger and attach its function name:
   ```go
   logger := log.Ctx(ctx).With().Str("func", "FunctionName").Logger()
   ```
4. **Dual Output (`MultiLevelWriter`)**: Logs must write simultaneously to both standard output (`os.Stdout`) and the persistent file `logs/app.log`.

---

## 2. HTTP Correlation Middleware (`X-Request-ID`)

Every incoming request must be enriched with a unique correlation identifier injected into the request header (`X-Request-ID`) and attached to a `zerolog.Logger` in `context.Context`.

### Standard Implementation (`internal/http/correlation_middleware.go`)
```go
package http

import (
    "net/http"
    "github.com/google/uuid"
    "github.com/rs/zerolog/log"
)

func CorrelationLoggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }

        // Echo back X-Request-ID header to client
        w.Header().Set("X-Request-ID", requestID)

        // Create request-scoped logger enriched with request_id
        reqLogger := log.With().
            Str("request_id", requestID).
            Logger()

        // Inject logger into request context
        ctx := reqLogger.WithContext(r.Context())

        // Emitting debug trace on request receipt
        reqLogger.Debug().
            Str("func", "CorrelationLoggerMiddleware").
            Str("method", r.Method).
            Str("path", r.URL.Path).
            Str("remote_addr", r.RemoteAddr).
            Msg("request received")

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## 3. Propagating Loggers Across Layers & Goroutines

### Within Synchronous Call Chains (`Handler -> Service -> Repository`)
Pass the HTTP request context (`r.Context()`) directly into the service method. Each layer extracts the logger via `log.Ctx(ctx)`:

```go
// Handler Level
func (s *Server) GetWeather(w http.ResponseWriter, r *http.Request) {
    logger := log.Ctx(r.Context()).With().Str("func", "GetWeather").Logger()
    logger.Info().Str("city", city).Msg("handling get weather request")
    
    // Pass r.Context() to service
    result, err := s.service.FetchWeatherFromAPIs(r.Context(), city)
    // ...
}

// Service Level
func (s *Service) FetchWeatherFromAPIs(ctx context.Context, city string) (*models.WeatherModel, error) {
    logger := log.Ctx(ctx).With().Str("func", "FetchWeatherFromAPIs").Logger()
    logger.Debug().Str("city", city).Msg("checking cache before live fetch")
    // ...
}
```

### Within Background & Concurrent Goroutines (`Fan-Out Workers`)
When spawning concurrent worker goroutines (`go func()`), **never pass a raw `context.Background()`** if you want correlation logs (`request_id`) to be preserved. Instead, pass the parent request context or derive a timeout context from it:

```go
func (s *Service) fetchWeatherConcurrently(ctx context.Context, city string) (*models.WeatherModel, error) {
    logger := log.Ctx(ctx).With().Str("func", "fetchWeatherConcurrently").Logger()
    logger.Debug().Msg("spawning concurrent API workers")

    // Derive timeout context from parent (inherits request_id logger!)
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    for _, apiFunc := range apis {
        wg.Add(1)
        go func(workerFunc func(context.Context, string) (*models.WeatherModel, error)) {
            defer wg.Done()
            // workerFunc extracts log.Ctx(ctx) inside -> request_id is automatically logged!
            if data, err := workerFunc(ctx, city); err != nil {
                errorCh <- err
            } else {
                resultCh <- data
            }
        }(apiFunc)
    }
    // ...
}
```

---

## 4. Logger Initialization Configuration (`internal/config/logger.go`)

Ensure the global logger is configured once at startup (`InitLogger()`) before the HTTP server starts:
```go
package config

import (
    "os"
    "path/filepath"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func InitLogger() {
    zerolog.SetGlobalLevel(zerolog.DebugLevel)
    zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

    logDir := "logs"
    _ = os.MkdirAll(logDir, 0755)

    logFilePath := filepath.Join(logDir, "app.log")
    lf, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        panic("failed to open log file: " + err.Error())
    }

    // Write concurrently to os.Stdout and logs/app.log
    multiWriters := zerolog.MultiLevelWriter(os.Stdout, lf)

    log.Logger = zerolog.New(multiWriters).With().Timestamp().Logger()
    zerolog.DefaultContextLogger = &log.Logger
}
```
