---
name: centralized_logging
description: Guidelines and setup instructions for centralized logging using Fluent-bit, Grafana Loki, and LogQL based on centralizedlog.md.
---

# Centralized Logging with Fluent-bit and Grafana Loki

This skill defines the standard practices for collecting, forwarding, and analyzing logs across applications and containers using Fluent-bit and Grafana Loki.

## 1. Log Collection with Fluent-bit

Fluent-bit is used to tail log files or receive container logs and forward them to destinations (like Grafana Loki).

### Tailing Log Files
To collect logs from an application writing to a file (e.g., `/app/logs/app.log`):

```ini
[SERVICE]
    Flush     1
    Log_Level info

[INPUT]
    Name  tail
    Path  /app/logs/app.log
    Tag   http-service
```

### Collecting Container Logs
To collect logs directly from container `stdout` (e.g., Nginx, Postgres), use Docker's `fluentd` logging driver in `docker-compose.yml`:

```yaml
services:
  nginx:
    image: nginx
    logging:
      driver: fluentd
      options:
        tag: nginx
        fluentd-sub-second-precision: 'true'
```

And configure Fluent-bit to receive forwarded logs:

```ini
[INPUT]
    Name forward
    Listen 0.0.0.0
    port 24224
```

## 2. Forwarding Logs to Grafana Loki

Use the Loki output plugin in Fluent-bit to forward collected logs. Ensure you apply appropriate tags and labels.

```ini
[OUTPUT]
    name        loki
    match       http-service
    host        loki
    port        3100
    labels      app=http-service
    drop_single_key true
    line_format key_value
```

## 3. Loki Setup & Grafana Data Source

**Loki Service Configuration (`docker-compose.yml`):**
```yaml
  loki:
    image: grafana/loki:2.9.2
    ports:
      - "3100:3100"
    volumes:
      - ./scripts/loki:/etc/loki
    command: -config.file=/etc/loki/config.yaml
```

**Grafana Data Source Configuration:**
```yaml
apiVersion: 1
datasources:
- name: Loki
  type: loki
  access: proxy
  orgId: 1
  url: http://loki:3100
  basicAuth: false
  isDefault: false
  version: 1
  editable: false
```

## 4. Querying and Metrics with LogQL

Use LogQL in Grafana Explore to analyze logs and extract metrics.

### Basic Text and Label Queries
- Search by keyword: `{app="http-service"} | json |~ "info"`
- Filter by parsed JSON field: `{app="http-service"} | json | level = "info"`

### Log Metrics Aggregation
- Count occurrences over time: `count_over_time({app="http-service"} |~ "debug" [1m])`
- Rate per second: `rate({app="http-service"}[1m])`
- Sum grouped by label: `sum by (level) (rate({app="http-service"} | json [1m]))`
- Top N results: `topk(1, sum by (level) (rate({app="http-service"} | json [1m])))`

### Unwrapping Values for Metrics
To calculate metrics (like average latency) from a value inside a structured log:

```logql
avg by (grpc_method, grpc_service, grpc_code)
  (avg_over_time({app="course-service"}
    | json
    | message = "finished call"
    | unwrap grpc_time_ms [1m]))
```