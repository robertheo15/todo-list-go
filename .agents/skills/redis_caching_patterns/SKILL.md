---
name: redis_caching_patterns
description: Guidelines and resilient patterns for using Redis (go-redis/v8) in the weather-api repository. Covers key normalization, TTL expiration, error wrapping, JSON serialization, and cache fallback behaviors.
---

# Redis Caching Patterns Skill (`redis_caching_patterns`)

This skill provides operational and architectural guardrails for interacting with **Redis** (`github.com/go-redis/redis/v8`) across the repository's repository layer (`internal/repository`).

## 1. Mandatory Caching Specifications

When writing cache keys or serialization routines:
1. **Case Normalization**: All keys derived from user input (`city`, `id`, `query`) **must** be normalized using `strings.ToLower()` to avoid duplicate entries and cache misses due to case differences (`Jakarta` vs `jakarta`).
2. **Key Namespacing**: Prefix cache keys with their domain namespace separated by colons (`weather:jakarta`, `forecast:london`).
3. **Time-To-Live (TTL)**: All cached items must set an explicit `EX` expiration. By project requirement, standard weather data must be cached for **12 hours** (`12 * time.Hour`).
4. **JSON Serialization**: Use `encoding/json` to serialize domain structs to byte slices before storing them with `r.redis.Set()`.

---

## 2. Proper Error Wrapping & Cache Miss Handling

When calling `r.redis.Get(ctx, key).Result()`, a cache miss is returned as the error `redis.Nil`.

### The Error Wrapping Trap (`CRITICAL`)
If you wrap `redis.Nil` inside a formatted string error (`errors.New(fmt.Sprintf("... %s", err.Error()))`), the underlying `redis.Nil` type is destroyed. Upstream service logic (`weather_service.go`) calling `errors.Is(err, redis.Nil)` will receive `false`, treating cache misses as database crashes!

#### Correct Implementation (`internal/repository/redis_repository.go`)
```go
func (r *RedisRepository) GetCachedRepository(ctx context.Context, key string, response *models.WeatherModel) error {
    normKey := strings.ToLower(key)
    val, err := r.redis.Get(ctx, normKey).Result()
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return redis.Nil // Pass through cache miss exactly
        }
        // Always wrap with %w so error chains remain intact
        return fmt.Errorf("redis.Get key=%s: %w", normKey, err)
    }

    if err := json.Unmarshal([]byte(val), response); err != nil {
        return fmt.Errorf("failed to unmarshal JSON from redis key=%s: %w", normKey, err)
    }
    return nil
}
```

---

## 3. Cache-First Service Fallback Pattern

In `internal/service/weather_service.go`, use the following fallback pattern when checking cache before making live external API calls:

```go
func (s *Service) FetchDataWithCache(ctx context.Context, key string) (*models.WeatherModel, error) {
    logger := log.Ctx(ctx).With().Str("func", "FetchDataWithCache").Logger()

    // 1. Attempt Cache Lookup
    var cached models.WeatherModel
    err := s.redisRepository.GetCachedRepository(ctx, key, &cached)
    if err == nil {
        logger.Info().Str("key", key).Msg("cache hit: serving data from redis")
        s.IncrementCacheHits()
        cached.Cached = true
        return &cached, nil
    }

    // 2. Inspect Error Type
    if !errors.Is(err, redis.Nil) {
        logger.Error().Err(err).Str("key", key).Msg("redis read error; falling back to live fetch")
    } else {
        logger.Debug().Str("key", key).Msg("cache miss: fetching live from APIs")
    }

    // 3. Live Fetch on Cache Miss or Cache Failure
    s.IncrementAPIUsage()
    liveData, fetchErr := s.fetchLiveFromExternalAPIs(ctx, key)
    if fetchErr != nil {
        return nil, fetchErr
    }

    // 4. Asynchronous / Non-Blocking Cache Write
    go func(data *models.WeatherModel) {
        bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if setErr := s.redisRepository.SetCachedRepository(bgCtx, data); setErr != nil {
            log.Error().Err(setErr).Str("key", key).Msg("failed to update redis cache")
        }
    }(liveData)

    return liveData, nil
}
```

---

## 4. Connection Pool & Ping Verification (`internal/config/redis.go`)

Ensure Redis is initialized with proper timeout options and verified with a `Ping` on application startup:
```go
func NewRedis(ctx context.Context) *redis.Client {
    logger := log.With().Str("func", "NewRedis").Logger()
    client := redis.NewClient(&redis.Options{
        Addr:         os.Getenv("REDIS_CLIENT"),
        PoolSize:     10,
        MinIdleConns: 2,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
    })

    if err := client.Ping(ctx).Err(); err != nil {
        logger.Fatal().Err(err).Msg("failed to connect to Redis on startup")
    }
    logger.Info().Msg("connected to Redis successfully")
    return client
}
```
