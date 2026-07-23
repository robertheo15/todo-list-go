---
name: clean_architecture_go
description: Guide and patterns for developing features, endpoints, and domain models adhering strictly to Go Clean Architecture and package boundary segregation in the weather-api repository. Use when adding new routes, domain models, or service/repository layers.
---

# Go Clean Architecture Skill (`clean_architecture_go`)

This skill provides mandatory architectural patterns, layer boundary rules, and step-by-step instructions for extending the **Weather API** (`weather-api`) codebase without violating dependency injection or package decoupling.

## 1. Package Dependency Flow & Layer Definitions

The repository is structured around unidirectional dependency boundaries:
```
cmd/main.go 
  │
  ▼ (wires dependencies and starts server)
internal/http/         [Presentation Layer] -> Depends on: internal/service, internal/models, pkg/http
  │
  ▼ (invokes domain workflows)
internal/service/      [Business Logic Layer] -> Depends on: internal/repository, internal/models
  │
  ▼ (fetches or persists data)
internal/repository/   [Data Access Layer]  -> Depends on: internal/models, external SDKs (redis/v8)
```

### Critical Layer Rules
1. **No Upstream Imports**: `internal/repository` and `internal/models` must **never** import `internal/service` or `internal/http`.
2. **No Handler Leakage**: `internal/service` must **never** accept or return `net/http` objects (`http.Request`, `http.ResponseWriter`) or Chi routing primitives.
3. **Pure Models**: `internal/models` must contain pure domain structs, DTOs, and mapping functions without external side effects, database connections, or HTTP clients.
4. **Shared Utilities**: Cross-cutting utilities (e.g., standard JSON response writers) belong in `pkg/http` or `internal/config`.

---

## 2. Step-by-Step Guide: Adding a New Feature or Endpoint

When asked to implement a new feature (e.g., `GET /api/v1/forecast`), follow this systematic 4-layer checklist:

### Step 1: Define Domain Model (`internal/models/`)
Create or update domain entities and external response schemas in `internal/models/`.
- Use clear `json` struct tags.
- Provide canonical conversion functions (e.g., `ExternalToDomainModel(resp ExternalResp) *DomainModel`).

```go
package models

type ForecastModel struct {
    City        string  `json:"city"`
    ForecastDay string  `json:"forecast_day"`
    TempMax     float64 `json:"temp_max"`
    TempMin     float64 `json:"temp_min"`
    Condition   string  `json:"condition"`
    Cached      bool    `json:"cached"`
}
```

### Step 2: Define Repository Methods (`internal/repository/`)
Add storage or caching methods to the repository struct (`RedisRepository`) or interface.
- Ensure methods accept `ctx context.Context` as their first argument.
- Return domain models `*models.ForecastModel` and explicit `error`.

```go
func (r *RedisRepository) SetCachedForecast(ctx context.Context, key string, data *models.ForecastModel) error {
    // Serialization & Redis SET logic
}
```

### Step 3: Implement Business Logic (`internal/service/`)
Add methods on `Service` (`internal/service/service.go`) orchestrating cache checks, external API requests, and business validation.
- Extract request logger: `logger := log.Ctx(ctx).With().Str("func", "GetForecast").Logger()`
- Return clean domain errors (or `fmt.Errorf("...: %w", err)`).

```go
func (s *Service) GetForecast(ctx context.Context, city string) (*models.ForecastModel, error) {
    logger := log.Ctx(ctx).With().Str("func", "GetForecast").Logger()
    // 1. Check cache via repository
    // 2. On miss, fetch live and store via repository
    return forecast, nil
}
```

### Step 4: Create HTTP Handler & Mount Route (`internal/http/`)
Add the handler method on `Server` (`internal/http/`) and mount it inside `http_server.go:Run()`.
- Validate query parameters or request body immediately.
- Invoke the `service` layer method using `r.Context()`.
- Return responses using `pkg/http/response.go` (`SetResponse` or `SetError`).

```go
func (s *Server) GetForecastHandler(w http.ResponseWriter, r *http.Request) {
    city := strings.TrimSpace(r.URL.Query().Get("city"))
    if city == "" {
        pkgHttp.SetError(w, http.StatusBadRequest, errors.New("city query parameter is required"))
        return
    }

    result, err := s.service.GetForecast(r.Context(), city)
    if err != nil {
        pkgHttp.SetError(w, http.StatusInternalServerError, err)
        return
    }

    pkgHttp.SetResponse(w, http.StatusOK, result, "Forecast fetched successfully", true)
}
```

---

## 3. Dependency Injection & Wiring (`cmd/main.go`)

When creating new repository interfaces or external clients, wire them explicitly in `cmd/main.go` using constructor injection:
```go
clientRedis := config.NewRedis(ctx)
repo := repository.NewRedisCacheRepository(clientRedis)
svc := service.NewService(repo)
server := http.NewServer(router, svc, ctx)
```
Do not use global singleton state for repositories or services.

---

## 4. Reference Templates
See `examples/handler_service_repo.go.template` in this skill folder for a complete end-to-end scaffolding template.
