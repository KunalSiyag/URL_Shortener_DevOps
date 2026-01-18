# 📚 Learning Journey - URL Shortener DevOps

This document captures the key learnings, errors encountered, and solutions discovered while building this URL shortener with full DevOps monitoring.

---

## 🐳 Docker & Container Issues

### 1. Docker Permission Denied

**Error:**
```
permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock
```

**Cause:** Docker requires elevated permissions to access the daemon socket.

**Solution:**
```bash
# Use sudo with docker commands
sudo docker compose up -d

# Or add user to docker group (requires logout/login)
sudo usermod -aG docker $USER
```

**Learning:** Always be aware of permission requirements when working with Docker. In production, use proper user permissions rather than running as root.

---

### 2. Port Already Allocated

**Error:**
```
Bind for 0.0.0.0:6379 failed: port is already allocated
```

**Cause:** Another process (or container) is already using the port.

**Solution:**
```bash
# Find what's using the port
lsof -i :6379
# or
sudo docker ps | grep 6379

# Stop the conflicting container
sudo docker stop <container_name>
```

**Learning:** Before starting services, check for port conflicts. Use `docker ps` to see running containers.

---

### 3. Address Already in Use (Go Server)

**Error:**
```
listen tcp :8080: bind: address already in use
```

**Cause:** The Go application tried to start while another process was already listening on port 8080.

**Solution:**
```bash
# Check what's using the port
lsof -i :8080
sudo docker ps | grep 8080
```

**Learning:** When a Docker container is running, you can't start the same app locally on the same port.

---

## 📊 Prometheus & Monitoring

### 4. Range Vector vs Instant Vector in Grafana

**Problem:** Query `promhttp_metric_handler_requests_total[1m]` works in Prometheus but not in Grafana.

**Cause:** Grafana visualizations require **instant vectors** (single value per series), but `[1m]` creates a **range vector** (multiple values over time).

**Solution:** Use `rate()` to convert range vector to instant vector:
```promql
# ❌ Won't work in Grafana panels
promhttp_metric_handler_requests_total[1m]

# ✅ Works in Grafana
rate(promhttp_metric_handler_requests_total[1m])
```

**Learning:**
- **Instant Vector:** Single value at a point in time → Works in graphs
- **Range Vector:** Multiple values over a time range → Must be converted

---

### 5. Understanding PromQL Functions

| Function | Purpose | Example |
|----------|---------|---------|
| `rate()` | Per-second average rate of increase | `rate(requests_total[5m])` |
| `sum()` | Aggregate across labels | `sum(requests_total)` |
| `histogram_quantile()` | Calculate percentiles | `histogram_quantile(0.95, rate(duration_bucket[5m]))` |
| `by()` | Group results by label | `sum by (status) (requests_total)` |

**Useful queries learned:**
```promql
# Request rate per second
rate(promhttp_metric_handler_requests_total[1m])

# 95th percentile latency
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))

# Average response time
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# Filter by status code
rate(prometheus_http_requests_total{status="200"}[1m])
```

---

## 🎨 Grafana Dashboard Creation

### 6. Creating Dashboards Manually

**Steps to create a Grafana dashboard:**
1. Open Grafana at http://localhost:3000
2. Login with admin/admin
3. Go to **Dashboards → New Dashboard**
4. Click **Add visualization**
5. Select **Prometheus** as data source
6. Enter your PromQL query
7. Customize visualization options
8. Save dashboard

**Dashboard panels created:**
- **Request Latency Histogram** - Shows p95 latency distribution
- **HTTP Requests** - Request rate over time
- **Error Rate** - Non-200 responses
- **Pods Status** - Service health indicators

![Grafana Dashboard](docs/grafana-dashboard.png)

---

### 7. Auto-Provisioning Datasources

**Learning:** Grafana can auto-configure datasources using provisioning files.

**File:** `grafana/provisioning/datasources/prometheus.yml`
```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

This eliminates manual datasource configuration!

---

## 🔧 Go Application Insights

### 8. Prometheus Metrics in Go

**Custom metrics defined:**

```go
// Counter - tracks total occurrences
shortenRequests = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "url_shortener_shorten_requests_total",
        Help: "Total number of shorten requests",
    },
    []string{"status"},
)

// Histogram - tracks request duration distribution
HttpRequestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request latency",
        Buckets: prometheus.DefBuckets,
    },
    []string{"method", "path"},
)
```

**Learning:** 
- Use **Counters** for things that only increase (requests, errors)
- Use **Histograms** for distributions (latency, sizes)
- Use **Gauges** for values that go up and down (connections, queue size)

---

### 9. Graceful Shutdown Pattern

```go
// Create shutdown signal channel
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

// Block until signal received
<-quit

// Shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
server.Shutdown(ctx)
```

**Learning:** Always implement graceful shutdown to:
- Complete in-flight requests
- Close database connections properly
- Flush logs and metrics

---

## 🐋 Docker Compose Learnings

### 10. Service Dependencies

```yaml
services:
  app:
    depends_on:
      redis:
        condition: service_healthy
```

**Learning:** Use `condition: service_healthy` with health checks to ensure dependencies are actually ready, not just started.

### 11. Container Networking

```yaml
networks:
  url-shortener-network:
    driver: bridge
```

**Learning:** Services on the same Docker network can reach each other by container name:
- `redis:6379` (not `localhost:6379`)
- `prometheus:9090`
- `app:8080`

---

## 📈 Key Takeaways

1. **Always use `rate()` for counters in Grafana** - Raw counters don't visualize well
2. **Docker networking uses container names** - Not localhost
3. **Health checks are essential** - For container orchestration
4. **Prometheus labels enable filtering** - Design metrics with useful labels
5. **Provisioning automates config** - Use for consistent deployments
6. **Graceful shutdown prevents data loss** - Handle SIGTERM properly

---

## 🔗 Resources Used

- [Prometheus Query Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/dashboards/)
- [Docker Compose Healthchecks](https://docs.docker.com/compose/compose-file/05-services/#healthcheck)
- [Go Prometheus Client](https://github.com/prometheus/client_golang)

---

*Last updated: January 18, 2026*
